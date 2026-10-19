package game

// The object tree (object.c): every record hangs off the location object (the world) or the
// non-world root (the player's things, quests...), as children of their parent, siblings in
// a list.

var (
	LocationObject  *Record
	CurrentLocation *Location
	NonworldRoot    *Record
	ObjectHeapFree  int32
	nextRecordID    int32

	foundObject        *Record
	objectFindIndex    int16
	objectFindGroup    uint16
	objectCountResult  int32
	objectSearchID     uint32
	objectSearchResult *Record
	objectFoundLast    *Record

	objectMoveOld, objectMoveNew [6]int32 // x, y, z, angle x, yaw, angle z
)

const recordHeaderSize = 71

// ObjectHeapInit makes the location object and the non-world root.
func ObjectHeapInit() {
	if ObjectHeapSize == 0 {
		ObjectHeapSize = 2048000
	}
	ObjectHeapFree = ObjectHeapSize
	LocationObject = ObjectAlloc(nil, nil, 48)
	LocationObject.Type = 1
	LocationObject.Image = 0xFFFF
	LocationObject.ID = uint32(0xFFFF0001) // -65535
	LocationObject.Location = &Location{}
	CurrentLocation = LocationObject.Location
	NonworldRoot = ObjectAlloc(nil, nil, 0)
	NonworldRoot.Type = 39
	NonworldRoot.ID = 700
}

// ObjectHeapShutdown drops the tree.
func ObjectHeapShutdown() { LocationObject, NonworldRoot, CurrentLocation = nil, nil, nil }

func objectFreeNode(o *Record) {
	if o.Twin != nil {
		o.Twin.Twin = nil
	}
	ObjectUnlink(o)
	objectHeapRelease(o)
}

// ObjectFreeChildren frees the object's children.
func ObjectFreeChildren(o *Record) {
	if o == nil {
		return
	}
	ObjectForeachPost(o.Children, objectFreeNode)
}

// ObjectFreeSingle frees the object (not its children); its next sibling.
func ObjectFreeSingle(o *Record) *Record {
	if o == nil {
		return nil
	}
	next := o.Next
	objectFreeNode(o)
	return next
}

// ObjectDelete frees the object and its children; its next sibling.
func ObjectDelete(o *Record) *Record {
	if o == nil {
		return nil
	}
	next := o.Next
	ObjectFreeChildren(o)
	ObjectFreeSingle(o)
	return next
}

// ObjectDetach unlinks the object; its parent.
func ObjectDetach(o *Record) *Record {
	if o == nil {
		return nil
	}
	parent := o.Parent
	ObjectUnlink(o)
	return parent
}

// ObjectAlloc makes a record with dataSize bytes of data (the heap's accounting), a copy of
// source's header and data or a new one with a new id, linked after `after`.
func ObjectAlloc(after, source *Record, dataSize int32) *Record {
	ObjectHeapFree -= dataSize + recordHeaderSize + 18
	var o *Record
	if source != nil {
		o = source.copyRecord()
		o.Next, o.Prev, o.Children, o.Parent = nil, nil, nil, nil
	} else {
		o = &Record{}
		o.ID = uint32(ObjectNewID(1))
	}
	o.DataSize = dataSize
	if after != nil {
		ObjectInsertAfter(after, o)
	}
	return o
}

func objectHeapRelease(o *Record) {
	ObjectHeapFree += o.DataSize + recordHeaderSize + 18
}

// ObjectClone copies the object (header and data) as a new child of its parent.
func ObjectClone(o *Record) *Record {
	c := ObjectCreateChild(o.Parent, nil, o.DataSize)
	links := [4]*Record{c.Next, c.Prev, c.Children, c.Parent}
	*c = *o.copyRecord()
	c.Next, c.Prev, c.Children, c.Parent = links[0], links[1], links[2], links[3]
	return c
}

// ObjectCreateChild makes a record as the parent's first child.
func ObjectCreateChild(parent, source *Record, dataSize int32) *Record {
	child := ObjectAlloc(parent.Children, source, dataSize)
	child.Parent = parent
	for s := child; s != nil; s = s.Prev {
		parent.Children = s
	}
	return child
}

// ObjectReparent moves the object under parent.
func ObjectReparent(parent, o *Record) *Record {
	ObjectUnlink(o)
	ObjectAddChild(parent, o)
	return o
}

// ObjectSetPosition places the object; its children follow the move.
func ObjectSetPosition(o *Record, x, y, z, angleX, yaw, angleZ int32) {
	objectMoveOld = [6]int32{o.X, o.Y, o.Z, int32(o.AngleX), int32(o.Yaw), int32(o.AngleZ)}
	o.X, o.Y, o.Z = x, y, z
	o.AngleX, o.Yaw, o.AngleZ = int16(angleX), int16(yaw), int16(angleZ)
	objectMoveNew = [6]int32{x, y, z, angleX, yaw, angleZ}
	if o.Type == 43 {
		return
	}
	ObjectForeach(o.Children, objectFollowMoveCb)
}

func objectFollowMoveCb(o *Record) {
	// the original compares the parent pointer with 3 and 18: never
	if o.Type == 52 || o.Parent.Type == 52 || o.Parent.Type == 22 {
		return
	}
	o.AngleX = int16((objectMoveNew[3] + (int32(o.AngleX) - objectMoveOld[3])) & 2047)
	o.Yaw = int16((objectMoveNew[4] + (int32(o.Yaw) - objectMoveOld[4])) & 2047)
	o.AngleZ = int16((objectMoveNew[5] + (int32(o.AngleZ) - objectMoveOld[5])) & 2047)
	dx := o.X - objectMoveOld[0]
	dz := o.Z - objectMoveOld[2]
	RotateXZ(&dx, &dz, int32(o.Yaw))
	o.X = objectMoveNew[0] + dx
	o.Y = objectMoveNew[1] + (o.Y - objectMoveOld[1])
	o.Z = objectMoveNew[2] + dz
}

// ObjectMoveBy moves the object by deltas.
func ObjectMoveBy(o *Record, dx, dy, dz, dAngleX, dYaw, dAngleZ int32) {
	ObjectSetPosition(o, o.X+dx, o.Y+dy, o.Z+dz, int32(o.AngleX)+dAngleX, int32(o.Yaw)+dYaw, int32(o.AngleZ)+dAngleZ)
}

// ObjectUnlink takes the object out of its list.
func ObjectUnlink(o *Record) {
	if o.Parent != nil && o.Parent.Children == o {
		o.Parent.Children = o.Next
	}
	if o.Prev != nil {
		o.Prev.Next = o.Next
	}
	if o.Next != nil {
		o.Next.Prev = o.Prev
	}
	o.Next, o.Prev, o.Parent = nil, nil, nil
}

// ObjectInsertAfter links o after `after`.
func ObjectInsertAfter(after, o *Record) {
	o.Next = after.Next
	if after.Next != nil {
		after.Next.Prev = o
	}
	after.Next = o
	o.Prev = after
	o.Parent = after.Parent
}

// ObjectAddChild links child under parent (after the first child).
func ObjectAddChild(parent, child *Record) {
	if parent.Children != nil {
		ObjectInsertAfter(parent.Children, child)
		return
	}
	parent.Children = child
	child.Parent = parent
	child.Prev, child.Next = nil, nil
}

// ObjectSwapSiblings swaps two objects of a list.
func ObjectSwapSiblings(first, second *Record) {
	if first.Next != second && second.Next != first {
		next, prev := second.Next, second.Prev
		if first.Prev != nil {
			first.Prev.Next = second
		}
		second.Next = first.Next
		second.Prev = first.Prev
		if second.Next != nil {
			second.Next.Prev = second
		}
		if second.Prev != nil {
			second.Prev.Next = first // as the original
		}
		first.Next = next
		first.Prev = prev
		if first.Next != nil {
			first.Next.Prev = first
		}
	} else {
		if second.Next == first {
			first, second = second, first
		}
		if first.Prev != nil {
			first.Prev.Next = second
		}
		if second.Next != nil {
			second.Next.Prev = first
		}
		second.Prev = first.Prev
		first.Next = second.Next
		first.Prev = second
		second.Next = first
	}
	if first.Prev == nil && first.Parent != nil {
		first.Parent.Children = first
	}
	if second.Prev != nil || second.Parent == nil {
		return
	}
	second.Parent.Children = second
}

// ObjectForeachNearPlayer calls fn on the open objects within maxDist of the player.
func ObjectForeachNearPlayer(o *Record, fn func(*Record), maxDist int32) {
	for ; o != nil; o = o.Next {
		if approxDist2D(o.X, o.Z, PlayerObject.X, PlayerObject.Z) < maxDist {
			fn(o)
			if o.Children != nil && o.Flags&1 == 0 {
				ObjectForeachNearPlayer(o.Children, fn, maxDist)
			}
		}
	}
}

// ObjectForeachPre calls fn on each object, then its children.
func ObjectForeachPre(o *Record, fn func(*Record)) {
	for o != nil {
		next := o.Next
		fn(o)
		if o.Children != nil {
			ObjectForeachPre(o.Children, fn)
		}
		o = next
	}
}

// ObjectForeachPost calls fn on each object's children, then it.
func ObjectForeachPost(o *Record, fn func(*Record)) {
	for o != nil {
		next := o.Next
		if o.Children != nil {
			ObjectForeachPost(o.Children, fn)
		}
		fn(o)
		o = next
	}
}

// ObjectForeach is ObjectForeachPost.
func ObjectForeach(o *Record, fn func(*Record)) { ObjectForeachPost(o, fn) }

// ObjectForeachSkipPlayer is ObjectForeachPost not entering type-4 objects.
func ObjectForeachSkipPlayer(o *Record, fn func(*Record)) {
	for o != nil {
		next := o.Next
		if o.Children != nil && o.Type != 4 {
			ObjectForeachSkipPlayer(o.Children, fn)
		}
		fn(o)
		o = next
	}
}

// ObjectForeachOpen calls fn on each object, then on the children of the open ones.
func ObjectForeachOpen(o *Record, fn func(*Record)) {
	for ; o != nil; o = o.Next {
		fn(o)
		if o.Children != nil && o.Flags&1 == 0 {
			ObjectForeachOpen(o.Children, fn)
		}
	}
}

// ObjectForeachUntil calls fn on each object, and on its children while fn says false.
func ObjectForeachUntil(o *Record, fn func(*Record) bool) {
	for o != nil {
		next := o.Next
		if !fn(o) && o.Children != nil {
			ObjectForeachUntil(o.Children, fn)
		}
		o = next
	}
}

// ObjectFind is whether fn says true for an object of the tree.
func ObjectFind(o *Record, fn func(*Record) bool) bool {
	for ; o != nil; o = o.Next {
		if fn(o) || ObjectFind(o.Children, fn) {
			return true
		}
	}
	return false
}

// ObjectFindOpen is ObjectFind not entering closed objects (below the first level).
func ObjectFindOpen(o *Record, fn func(*Record) bool) bool {
	for ; o != nil; o = o.Next {
		if fn(o) {
			return true
		}
		if o.Children != nil && o.Flags&1 == 0 && ObjectFind(o.Children, fn) {
			return true
		}
	}
	return false
}

func objectFindItemCb(o *Record) {
	if objectFindIndex == -1 || o.Type != 2 {
		return
	}
	if o.Item.Group != objectFindGroup || o.Item.Index != uint16(objectFindIndex) {
		return
	}
	objectFindIndex = -1
	foundObject = o
}

// ObjectFindType is the first object of the type.
func ObjectFindType(root *Record, typ int32) *Record {
	foundObject = nil
	objectFindIndex = int16(typ)
	ObjectFind(root, func(o *Record) bool {
		if int16(o.Type) != objectFindIndex {
			return false
		}
		foundObject = o
		return true
	})
	return foundObject
}

// ObjectCountType counts the objects of the type.
func ObjectCountType(root *Record, typ int16) int32 {
	objectFindIndex = typ
	objectCountResult = 0
	ObjectForeach(root, func(o *Record) {
		if int16(o.Type) == objectFindIndex {
			objectCountResult++
		}
	})
	return objectCountResult
}

// ObjectCreateInBlock makes an object of the location (with the location's next id).
func ObjectCreateInBlock(parent *Record, typ uint8, dataSize int32, image uint16, misc13 uint16) *Record {
	var o *Record
	if parent != nil {
		o = ObjectCreateChild(parent, nil, dataSize)
	} else {
		o = ObjectAlloc(nil, nil, dataSize)
	}
	o.Type = typ
	o.Image = image
	o.Misc13 = misc13
	o.ID = LocationObject.ID + uint32(CurrentLocation.ObjectCounter)
	CurrentLocation.ObjectCounter++
	return o
}

// ObjectFindByID finds the object with the id under root, or (root nil) in the world, then
// outside it.
func ObjectFindByID(root *Record, id uint32) *Record {
	objectSearchID = id
	objectSearchResult = nil
	cb := func(o *Record) bool {
		if o.ID == objectSearchID {
			objectSearchResult = o
		}
		return objectSearchResult != nil
	}
	if root == nil {
		ObjectForeach(LocationObject, func(o *Record) { cb(o) })
		objectFoundLast = objectSearchResult
		if objectSearchResult != nil {
			return objectSearchResult
		}
		ObjectForeach(NonworldRoot, func(o *Record) { cb(o) })
		objectFoundLast = objectSearchResult
		return objectSearchResult
	}
	ObjectFind(root, cb)
	return objectSearchResult
}

// ObjectRandomChildOfType is a random child of the type.
func ObjectRandomChildOfType(parent *Record, typ int32) *Record {
	n := ObjectCountType(parent.Children, int16(typ))
	if n == 0 {
		return nil
	}
	foundObject = nil
	k := Rand() % n
	t := uint8(typ)
	ObjectFind(parent.Children, func(o *Record) bool {
		if o.Type != t {
			return false
		}
		if k == 0 {
			foundObject = o
			return true
		}
		k--
		return false
	})
	return foundObject
}

// ObjectDeleteBlock frees the objects whose id is in the block's range.
func ObjectDeleteBlock(root *Record, blockID uint32) {
	blockID &= 0xFFFF0000
	ObjectForeachPost(root, func(o *Record) {
		if o.ID&0xFFFF0000 != blockID {
			return
		}
		if o.Twin != nil {
			o.Twin.Twin = nil
		}
		ObjectFreeSingle(o)
	})
}

// ObjectNewID is a free id with the high word.
func ObjectNewID(high int32) int32 {
	if nextRecordID >= 63000 {
		nextRecordID = 10000
	}
	for {
		id := high<<16 + nextRecordID
		nextRecordID++
		if (LocationObject == nil || ObjectFindByID(LocationObject, uint32(id)) == nil) &&
			(NonworldRoot == nil || ObjectFindByID(NonworldRoot, uint32(id)) == nil) {
			return id
		}
	}
}

// ObjectDeleteQuestObjects frees the quest's objects (a quest giver only loses its link).
func ObjectDeleteQuestObjects(root *Record, questID uint8) {
	ObjectForeachPost(root, func(o *Record) {
		if o.QuestID != questID {
			return
		}
		if o.Type == 8 && o.Person.Flags&128 != 0 {
			o.Twin.Twin = nil
			o.Twin = nil
			o.QuestID = 0
			return
		}
		if len(LoadedLocation.Doors) != 0 && o.ID&0xFFFF0000 == LocationObject.ID&0xFFFF0000 {
			if d := LocationFindDoor(o.ID); d != nil {
				d.Flags &= 0xFFF
			}
		}
		UnequipObject(o)
		ObjectDelete(o)
	})
}

// ObjectTreeSize is the data bytes of the tree.
func ObjectTreeSize(root *Record) int32 {
	n := int32(0)
	ObjectForeach(root, func(o *Record) { n += o.DataSize + recordHeaderSize })
	return n
}

// ObjectDeleteType frees the objects of the type.
func ObjectDeleteType(root *Record, typ uint8) {
	ObjectForeachPost(root, func(o *Record) {
		if o.Type == typ {
			ObjectDelete(o)
		}
	})
}

// ObjectFindQuest is the first object of the quest.
func ObjectFindQuest(root *Record, questID uint8) *Record {
	foundObject = nil
	ObjectFind(root, func(o *Record) bool {
		if o.QuestID == questID {
			foundObject = o
			return true
		}
		return false
	})
	return foundObject
}

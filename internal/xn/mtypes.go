package xn

// The ARCH3D model records (xnstruct.h). A model keeps its record's bytes and header (its
// lists by their offsets, as preparing and the frames change them); its faces are parsed into
// Go once, and preparing fills its points by index, its normals and its texture axes.

// ModelFacePoint is a point of a face: its vertex (the record's byte offset, index * 12;
// after prepare the index) and the file's u, v (points 1 and 2: deltas from the previous).
type ModelFacePoint struct {
	Vertex int32
	U, V   int16
}

// ModelFaceData is a face's texture axes: u and v gradients in object space.
type ModelFaceData struct {
	UAxis, VAxis Vec3
}

// ModelFace is a face of a model.
type ModelFace struct {
	Offset    int32 // its byte offset in the record (its identity)
	Index     int32 // its index: the normal and face data
	Shade     uint8 // darkening in shade rows
	Texture   uint16
	Unknown04 int32
	Points    []ModelFacePoint

	// after prepare (the file's slots of points 0, 1, 2)
	UVPacked uint32 // (u | v << 16) << 4
	PlaneD   int32  // dot(point 0, normal)
	Data     *ModelFaceData
}

// Model is an ARCH3D record.
type Model struct {
	Raw []byte // the record

	Version          uint32
	PointCount       int32
	FaceCount        int32
	Radius           int32
	FrameCount       int32
	FrameTableOffset int32
	FaceDataOffset   int32
	SphereOffset     int32
	SphereCount      int32
	Unknown24        int32
	PointOffset      int32
	NormalOffset     int32
	Unknown38        int32
	FaceOffset       int32

	Faces         []*ModelFace
	Points        []Vec3 // by index, after prepare
	Normals       []Vec3 // by face, after prepare (8-bit fraction)
	FaceData      []ModelFaceData
	Spheres       []ModelSphere // the collision spheres (parsed on first use)
	spheresParsed bool
}

// ModelSphereFace is a collision sphere's face: its offset and its index * 4.
type ModelSphereFace struct {
	Face    int32
	Normal4 uint16
}

// ModelSphere is a collision sphere: centre and radius in object space (24.8), the faces
// near it, descending by offset.
type ModelSphere struct {
	X, Y, Z, Radius int32
	Faces           []ModelSphereFace
}

// ModelHandle is a placement of a model.
type ModelHandle struct {
	Model                           *Model
	Lights                          []LightRef // this frame's light list
	Matrix                          int32      // this frame's slot in RenderMatrixPool
	Pad0C                           int16
	BaseAngleX, BaseYaw, BaseAngleZ int16
	RelX, RelY, RelZ                int32
	X, Y, Z                         int32
	AngleX, Yaw, AngleZ             int32
	Frame                           uint8
	Flags                           uint8
}

// MatrixPool is the frame's object matrices: view[i] (the gradients) and light[i].
type MatrixPool struct {
	View  [200]Mat3
	Light [200]Mat3
}

// RenderMatrixPool is the frame's matrix pool.
var RenderMatrixPool MatrixPool

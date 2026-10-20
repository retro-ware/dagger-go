package game

// A quest record's data (type 14) is its QBN file with the sections' state written in
// (questdata.go).

func questToDisk(q *Quest) []byte { return encodeQuest(q) }

func questFromDisk(b []byte) *Quest { return decodeQuest(b, true) }

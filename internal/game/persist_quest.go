package game

// A quest record's data (type 14) is its QBN file with the sections' state written in. Until
// the quest modules lay their structs out as the file does, the bytes go as the quest holds
// them (Raw).

func questToDisk(q *Quest) []byte { return append([]byte(nil), q.Raw...) }

func questFromDisk(b []byte) *Quest { return &Quest{Raw: append([]byte(nil), b...)} }

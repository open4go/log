package model

import "go.mongodb.org/mongo-driver/bson"

func mergeBsonM(m1, m2 bson.M) bson.M {
	merged := bson.M{}

	// 合并第一个 map
	for k, v := range m1 {
		merged[k] = v
	}

	// 合并第二个 map，覆盖冲突的键
	for k, v := range m2 {
		merged[k] = v
	}

	return merged
}

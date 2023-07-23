package category

import (
	rtime "github.com/r2day/base/time"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"time"
)

// ResourceName 返回资源名称
func (m *Model) ResourceName() string {
	return modelName
}

// CollectionName 返回表名称
func (m *Model) CollectionName() string {
	return collectionNamePrefix + modelName + collectionNameSuffix
}

// IncrementReference 更新
// https://www.mongodb.com/docs/manual/reference/operator/update/inc/
func (m *Model) IncrementReference(id string) error {

	coll := m.Meta.Handler.Collection(m.Meta.Collection)
	objID, _ := primitive.ObjectIDFromHex(id)
	filter := bson.D{{Key: "_id", Value: objID}}
	// 设定更新时间
	m.Meta.UpdatedAt = rtime.FomratTimeAsReader(time.Now().Unix())

	result, err := coll.UpdateOne(m.Meta.Context, filter,
		bson.D{{Key: "$set", Value: bson.D{{"reference", 1}}}})
	if err != nil {
		return err
	}

	if result.MatchedCount < 1 {
		return nil
	}
	return nil
}

// DecrementReference 更新
// https://www.mongodb.com/docs/manual/reference/operator/update/inc/
func (m *Model) DecrementReference(id string) error {
	coll := m.Meta.Handler.Collection(m.Meta.Collection)
	objID, _ := primitive.ObjectIDFromHex(id)
	filter := bson.D{{Key: "_id", Value: objID}}
	// 设定更新时间
	m.Meta.UpdatedAt = rtime.FomratTimeAsReader(time.Now().Unix())

	result, err := coll.UpdateOne(m.Meta.Context, filter,
		bson.D{{Key: "$set", Value: bson.D{{"reference", -1}}}})
	if err != nil {
		return err
	}

	if result.MatchedCount < 1 {
		return nil
	}
	return nil
}

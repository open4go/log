package model

import (
	"context"
	rtime "github.com/r2day/base/time"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"time"
)

// MetaModel 元模型
type MetaModel struct {
	// 上下文
	Context context.Context `json:"-" bson:"-"`
	// 数据库
	Handler *mongo.Database `json:"-" bson:"-"`
	// 表名称
	Collection string `json:"-" bson:"-"`
	// 商户号
	MerchantID string `json:"merchant_id" bson:"merchant_id"`
	// 创建者
	AccountID string `json:"account_id" bson:"account_id"`
	// 创建时间
	CreatedAt string `json:"created_at" bson:"created_at"`
	// 更新时间
	UpdatedAt string `json:"updated_at" bson:"updated_at"`
	// 状态
	Status bool `json:"status"`
	// 根据角色的最低级别写入
	AccessLevel uint `json:"access_level" bson:"access_level"`
}

// Model 模型
type Model struct {
	// 基本的数据库模型字段，一般情况所有model都应该包含如下字段
	Meta MetaModel `json:"meta" bson:"meta"`
}

// Init 设置名称
func (m *Model) Init(ctx context.Context, handler *mongo.Database, name string) *Model {
	m.Meta.Context = ctx
	m.Meta.Handler = handler
	m.Meta.Collection = name
	return m
}

// Create 创建
func (m *Model) Create(d interface{}) (string, error) {
	// 保存时间设定
	m.Meta.CreatedAt = rtime.FomratTimeAsReader(time.Now().Unix())
	// 更新时间设定
	m.Meta.UpdatedAt = rtime.FomratTimeAsReader(time.Now().Unix())

	coll := m.Meta.Handler.Collection(m.Meta.Collection)
	// 插入记录
	result, err := coll.InsertOne(m.Meta.Context, d)
	if err != nil {
		return "", err
	}
	id := result.InsertedID.(primitive.ObjectID).Hex()
	return id, nil
}

// Delete 删除
// delete	DELETE http://my.api.url/posts/123
func (m *Model) Delete(id string) error {
	// 更新时间设定
	m.Meta.UpdatedAt = rtime.FomratTimeAsReader(time.Now().Unix())

	coll := m.Meta.Handler.Collection(m.Meta.Collection)
	objID, _ := primitive.ObjectIDFromHex(id)
	filter := bson.D{{Key: "_id", Value: objID}}
	// 执行删除
	result, err := coll.DeleteOne(m.Meta.Context, filter)

	if err != nil {
		return err
	}

	if result.DeletedCount < 1 {
		return nil
	}
	return nil
}

// GetOne 详情
// getOne	GET http://my.api.url/posts/123
func (m *Model) GetOne(d interface{}, id string) error {
	coll := m.Meta.Handler.Collection(m.Meta.Collection)
	objID, _ := primitive.ObjectIDFromHex(id)
	filter := bson.D{{Key: "_id", Value: objID}}
	err := coll.FindOne(m.Meta.Context, filter).Decode(d)
	if err != nil {
		return err
	}
	return nil
}

// GetBy 通过自定义查询字段
// getOne	GET http://my.api.url/posts/123
func (m *Model) GetBy(d interface{}, filter interface{}) error {
	coll := m.Meta.Handler.Collection(m.Meta.Collection)
	err := coll.FindOne(m.Meta.Context, filter).Decode(d)
	if err != nil {
		return err
	}
	return nil
}

// Update 更新
// update	PUT http://my.api.url/posts/123
func (m *Model) Update(d interface{}, id string) error {
	coll := m.Meta.Handler.Collection(m.Meta.Collection)
	objID, _ := primitive.ObjectIDFromHex(id)
	filter := bson.D{{Key: "_id", Value: objID}}
	// 设定更新时间
	m.Meta.UpdatedAt = rtime.FomratTimeAsReader(time.Now().Unix())

	result, err := coll.UpdateOne(m.Meta.Context, filter, bson.D{{Key: "$set", Value: d}})
	if err != nil {
		return err
	}

	if result.MatchedCount < 1 {
		return err
	}

	return nil
}

// GetList 获取列表
// getList	GET http://my.api.url/posts?sort=["title","ASC"]&range=[0, 24]&filter={"title":"bar"}
func (m *Model) GetList(filter interface{}, d interface{}) (int64, error) {
	coll := m.Meta.Handler.Collection(m.Meta.Collection)
	// 声明需要返回的列表
	//results := make([]*Model, 0)
	// 获取总数（含过滤规则）
	totalCounter, err := coll.CountDocuments(context.TODO(), filter)
	if err == mongo.ErrNoDocuments {
		return 0, err
	}
	// 获取数据列表
	cursor, err := coll.Find(m.Meta.Context, filter)
	if err == mongo.ErrNoDocuments {
		return totalCounter, err
	}

	if err != nil {
		return totalCounter, err
	}

	if err = cursor.All(context.TODO(), d); err != nil {
		return totalCounter, err
	}
	return totalCounter, nil
}

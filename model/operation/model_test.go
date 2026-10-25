package operation

import (
	"testing"

	"go.mongodb.org/mongo-driver/bson"
)

// 旧文档没有新字段时必须能正常解码，新字段保持空值。
func TestLegacyDocumentDecodes(t *testing.T) {
	raw, err := bson.Marshal(bson.M{"operator": "张三", "device": "Mozilla/5.0", "client_ip": "1.2.3.4"})
	if err != nil {
		t.Fatal(err)
	}
	var m Model
	if err := bson.Unmarshal(raw, &m); err != nil {
		t.Fatal(err)
	}
	if m.Client != "" || m.ClientVersion != "" || m.IPLocation != "" || m.DeviceID != "" {
		t.Fatalf("new fields should be empty for legacy docs: %+v", m)
	}
	if m.Operator != "张三" {
		t.Fatalf("operator=%q", m.Operator)
	}
}

func TestNewFieldsRoundTrip(t *testing.T) {
	in := Model{Client: "x9", ClientVersion: "0.0.118", DeviceID: "d-1", IPLocation: "广西南宁 电信"}
	raw, err := bson.Marshal(in)
	if err != nil {
		t.Fatal(err)
	}
	var out Model
	if err := bson.Unmarshal(raw, &out); err != nil {
		t.Fatal(err)
	}
	if out.Client != "x9" || out.ClientVersion != "0.0.118" || out.DeviceID != "d-1" || out.IPLocation != "广西南宁 电信" {
		t.Fatalf("round trip: %+v", out)
	}
}

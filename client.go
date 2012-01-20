package couchbase

import (
	"encoding/json"
	"fmt"
	"math/rand"
	"net/http"
	"net/url"
)

func (b *Bucket) getVBInfo(k string) (*memcachedClient, uint16) {
	vb := b.VBHash(k)
	masterId := b.VBucketServerMap.VBucketMap[vb][0]
	if b.connections[masterId] == nil {
		b.connections[masterId] = connect("tcp", b.VBucketServerMap.ServerList[masterId])
	}
	return b.connections[masterId], uint16(vb)
}

// Set a value in this bucket.
// The value will be serialized into a JSON document.
func (b *Bucket) Set(k string, v interface{}) error {
	mc, vb := b.getVBInfo(k)
	data, err := json.Marshal(v)
	if err != nil {
		return err
	}
	res := mc.Set(vb, k, 0, 0, data)
	if res.Status != mcSUCCESS {
		return res
	}
	return nil
}

// Get a value from this bucket.
// The value is expected to be a JSON stream and will be deserialized
// into rv.
func (b *Bucket) Get(k string, rv interface{}) error {
	mc, vb := b.getVBInfo(k)
	res := mc.Get(vb, k)
	if res.Status != mcSUCCESS {
		return res
	}
	return json.Unmarshal(res.Body, rv)
}

// Delete a key from this bucket.
func (b *Bucket) Delete(k string) error {
	mc, vb := b.getVBInfo(k)
	res := mc.Del(vb, k)
	if res.Status != mcSUCCESS {
		return res
	}
	return nil
}

type ViewRow struct {
	ID    string
	Key   interface{}
	Value interface{}
	Doc   *interface{}
}

type ViewResult struct {
	TotalRows int `json:"total_rows"`
	Rows      []ViewRow
}

// Execute a view
func (b *Bucket) View(ddoc, name string, params map[string]interface{}) (vres ViewResult, err error) {
	// Pick a random node to service our request.
	node := b.Nodes[rand.Intn(len(b.Nodes))]
	u, err := url.Parse(node.CouchAPIBase)
	if err != nil {
		return ViewResult{}, err
	}

	values := url.Values{}
	for k, v := range params {
		values[k] = []string{fmt.Sprintf("%v", v)}
	}

	u.Path = fmt.Sprintf("/%s/_design/%s/_view/%s", b.Name, ddoc, name)
	u.RawQuery = values.Encode()

	res, err := http.Get(u.String())
	if err != nil {
		return ViewResult{}, err
	}
	defer res.Body.Close()

	d := json.NewDecoder(res.Body)
	if err = d.Decode(&vres); err != nil {
		return ViewResult{}, err
	}
	return vres, nil

}

package passer

import (
	"context"
	"log"
	"testing"
)

type ptype struct {
	Key string
}

type presult struct {
	Key string
}

func TestCase1(t *testing.T) {
	pr := NewPasser[*presult]()
	pr.RegisterPasser(&ptype{}, func(ctx context.Context, obj any) (*presult, error) {
		log.Println(obj)
		v := obj.(*ptype)

		if v.Key != "haha" {
			t.Error(v.Key)
		}

		pt := presult{
			Key: v.Key + "+result",
		}
		return &pt, nil
	})
	data, err := pr.PackToBytes(&ptype{
		Key: "haha",
	})
	if err != nil {
		panic(err)
	}

	r, err := pr.ExecuteWithBytes(context.TODO(), data)
	if err != nil {
		t.Error(err)
	}

	if r.Key != "haha+result" {
		t.Error(r.Key != "haha+result")
	}
}

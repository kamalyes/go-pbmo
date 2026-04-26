/*
 * @Author: kamalyes 501893067@qq.com
 * @Date: 2026-04-23 00:00:00
 * @LastEditors: kamalyes 501893067@qq.com
 * @LastEditTime: 2026-04-23 00:00:00
 * @FilePath: \go-pbmo\generic_test.go
 * @Description: 泛型便捷函数测试
 *
 * Copyright (c) 2026 by kamalyes, All Rights Reserved.
 */

package pbmo

import (
	"github.com/stretchr/testify/assert"
	"testing"
)

func TestGenericRegister(t *testing.T) {
	c := Register[TestModel, TestPB]()
	assert.NotNil(t, c)

	c2 := Register[TestModel, TestPB]()
	assert.Same(t, c, c2, "should return same converter instance")
}

func TestGenericRegisterWith(t *testing.T) {
	c := RegisterWith[TestSimpleModel, TestSimplePB](WithAutoTimeConversion(false))
	assert.NotNil(t, c)
}

func TestGenericToPB(t *testing.T) {
	m := &TestModel{
		ID:     1,
		Name:   "test",
		Email:  "test@example.com",
		Age:    25,
		Score:  98.5,
		Active: true,
		Tags:   []string{"go", "pb"},
	}

	pb, err := ToPB[TestModel, TestPB](m)
	assert.NoError(t, err)
	assert.Equal(t, uint64(1), pb.Id)
	assert.Equal(t, "test", pb.Name)
	assert.Equal(t, "test@example.com", pb.Email)
	assert.Equal(t, int32(25), pb.Age)
	assert.Equal(t, 98.5, pb.Score)
	assert.Equal(t, true, pb.Active)
}

func TestGenericFromPB(t *testing.T) {
	pb := &TestPB{
		Id:     2,
		Name:   "from_pb",
		Email:  "pb@example.com",
		Age:    30,
		Score:  88.0,
		Active: false,
		Tags:   []string{"proto"},
	}

	m, err := FromPB[TestPB, TestModel](pb)
	assert.NoError(t, err)
	assert.Equal(t, uint64(2), m.ID)
	assert.Equal(t, "from_pb", m.Name)
	assert.Equal(t, "pb@example.com", m.Email)
	assert.Equal(t, 30, m.Age)
}

func TestGenericConverterFor(t *testing.T) {
	c := ConverterFor[TestModel, TestPB]()
	assert.NotNil(t, c)
	assert.Contains(t, c.GetPBType().String(), "TestPB")
	assert.Contains(t, c.GetModelType().String(), "TestModel")
}

func TestGenericToPBNilModel(t *testing.T) {
	result, err := ToPB[TestModel, TestPB](nil)
	assert.NoError(t, err)
	assert.Nil(t, result)
}

func TestGenericFromPBNilPB(t *testing.T) {
	result, err := FromPB[TestPB, TestModel](nil)
	assert.NoError(t, err)
	assert.Nil(t, result)
}

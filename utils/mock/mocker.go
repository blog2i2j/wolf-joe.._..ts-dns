package mock

import (
	"reflect"

	"github.com/agiledragon/gomonkey"
)

// Mocker gomonkey的封装
type Mocker struct {
	patches []*gomonkey.Patches
}

// FuncSeq gomonkey.ApplyFuncSeq的封装
func (m *Mocker) FuncSeq(target interface{}, outputs []gomonkey.Params) {
	var cells []gomonkey.OutputCell
	for _, output := range outputs {
		cells = append(cells, gomonkey.OutputCell{Values: output})
	}
	m.patches = append(m.patches, gomonkey.ApplyFuncSeq(target, cells))
}

// Func gomonkey.ApplyFunc的封装
func (m *Mocker) Func(target interface{}, double interface{}) {
	m.patches = append(m.patches, gomonkey.ApplyFunc(target, double))
}

// MethodSeq gomonkey.ApplyMethodSeq的封装
func (m *Mocker) MethodSeq(target interface{}, method string, outputs []gomonkey.Params) {
	var cells []gomonkey.OutputCell
	for _, output := range outputs {
		cells = append(cells, gomonkey.OutputCell{Values: output})
	}
	p := gomonkey.ApplyMethodSeq(reflect.TypeOf(target), method, cells)
	m.patches = append(m.patches, p)
}

// Method gomonkey.ApplyMethod的封装
func (m *Mocker) Method(target interface{}, method string, double interface{}) {
	t := reflect.TypeOf(target)
	m.patches = append(m.patches, gomonkey.ApplyMethod(t, method, double))
}

// Reset 重置所有mock
func (m *Mocker) Reset() {
	for _, patches := range m.patches {
		patches.Reset()
	}
	m.patches = []*gomonkey.Patches{}
}

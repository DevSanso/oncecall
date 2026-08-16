package vm

import (
	"oncecall/utils"
	"oncecall/utils/generic"
	"reflect"
	"sync"

	"github.com/Shopify/go-lua"
	"go.uber.org/zap"
)

type luaVM struct {
	raw  *lua.State
	once sync.Once

	input  [][]any
	output [][]any

	tempGPtrMap *generic.GenericSyncMap[string, any]
}

type LuaVm Vm[lua.State]

func NewLuaVM() LuaVm {
	obj := &luaVM{
		raw:         lua.NewState(),
		once:        sync.Once{},
		input:       nil,
		output:      nil,
		tempGPtrMap: nil,
	}

	obj.luaInit()
	obj.registerGetDataLuaFunc("oncecall_get_data")
	obj.registerPutDataLuaFunc("oncecall_put_data")
	obj.registerCacheMapLuaUserData("global_map")
	return obj
}
func (w *luaVM) luaInit() {
	lua.OpenLibraries(w.raw)
}

func (*luaVM) convertLuaToAsGo(l *lua.State, index int) (output any, err error) {
	switch {
	case l.IsTable(index):
		size := l.RawLength(index)
		
		buf := make([]byte, size)
		for idx := 0; idx < size; idx += 1 {
			l.RawGetInt(index, idx+1)
			byteData, byteOk := l.ToInteger(-1)
			l.Pop(1)
			if !byteOk {
				err = utils.ErrorfPc("table element type is not number [%s]", lua.TypeNameOf(l, -1))
				buf = nil
				break
			}
			
			if byteData > 256 || byteData < 0 {
				err = utils.ErrorfPc("table only support byte array [idx:%d, value:%d]", idx+1, byteData)
				buf = nil
				break
			}

			buf[idx] = byte(byteData)
		}
		output = buf
	case l.IsNumber(index):
		v, _ := l.ToNumber(index)
		output = v
	case l.IsBoolean(index):
		output = l.ToBoolean(index)
	case l.IsNil(index):
		output = nil
	case l.IsString(-1):
		v, _ := l.ToString(index)
		output = v
	default:
		err = utils.ErrorfPc("unsupported type at [%d]", index)
	}
	l.Pop(1)
	return
}

func (*luaVM) pushLuaFromGo(l *lua.State, v any) error {
	switch x := v.(type) {
	case nil:
		l.PushNil()
	case string:
		l.PushString(x)
	case int:
		l.PushInteger(x)
	case int64:
		l.PushInteger(int(x))
	case float64:
		l.PushNumber(x)
	case bool:
		l.PushBoolean(x)
	case []byte:
		l.NewTable()
		for dataIdx, data := range x {
			l.PushInteger(int(data))
			l.RawSetInt(-2, dataIdx+1)
		}
	default:
		return utils.ErrorfPc("luavm not support type %s", reflect.TypeOf(v).Name())
	}

	return nil
}

func (w *luaVM) registerGetDataLuaFunc(name string) {
	w.raw.PushGoFunction(func(l *lua.State) int {
		data := w.input
		isCastFailFlag := false

		l.CreateTable(len(data), 0)

	rootLoop:
		for i, row := range data {
			l.CreateTable(len(row), 0)

			for j, v := range row {
				if err := w.pushLuaFromGo(l, v); err != nil {
					isCastFailFlag = true
					break rootLoop
				}
				l.RawSetInt(-2, j+1)
			}

			l.RawSetInt(-2, i+1)
		}

		if isCastFailFlag {
			lua.Errorf(l, "%s", utils.ErrorfPc("cast failed type"))
			return 0
		} else {
			return 1
		}
	})
	w.raw.SetGlobal(name)
}

func (w *luaVM) registerPutDataLuaFunc(name string) {
	w.raw.PushGoFunction(func(l *lua.State) int {
		if !l.IsTable(-1) {
			lua.Errorf(l, "%s", utils.ErrorfPc("row %s is not a table %s", name, w.raw.TypeOf(-1).String()))
			l.Pop(-1)
			return 0
		}

		defer l.Pop(-1)
		rowLen := l.RawLength(-1)
		var err error = nil
		w.output = make([][]any, rowLen)

		for rowIdx := 1; rowIdx <= rowLen; rowIdx++ {
			l.RawGetInt(-1, rowIdx)

			if !l.IsTable(-1) {
				l.Pop(-1)
				err = utils.ErrorfPc("row %d is not a table", rowIdx)
				break
			}

			dataLen := l.RawLength(-1)
			row := make([]any, dataLen)

			for idx := 1; idx <= dataLen; idx++ {
				l.RawGetInt(-1, idx)

				row[idx-1], err = w.convertLuaToAsGo(l, -1)
				if err != nil {
					break
				}
			}

			w.output[rowIdx-1] = row
			l.Pop(1)
			if err != nil {
				break
			}
		}

		if err != nil {
			lua.Errorf(l, "%s", err.Error())
		}
		return 0
	})

	w.raw.SetGlobal(name)
}

func (w *luaVM) registerCacheMapLuaUserData(userdataName string) {

	w.raw.NewTable()
	w.raw.PushGoFunction(func(l *lua.State) int {
		key, ok := l.ToString(1)
		if !ok {
			err := utils.ErrorfPc("can't cast type string")
			zap.L().Error(err.Error())
			return 0
		}
		data, ok := w.tempGPtrMap.Load(key)

		if ok {
			l.PushLightUserData(data)
		} else {
			l.PushNil()
		}

		return 1
	})

	w.raw.SetField(-2, "get")

	w.raw.PushGoFunction(func(l *lua.State) int {
		key, kOk := l.ToString(1)
		if !kOk {
			err := utils.ErrorfPc("can't cast type string [%v]", kOk)
			zap.L().Error(err.Error())
			return 0
		}

		if l.IsNil(2) {
			err := utils.ErrorfPc("can't store nil [%v]", key)
			zap.L().Error(err.Error())
			return 0
		}

		val := l.ToValue(2)
		w.tempGPtrMap.Store(key, val)
		return 0
	})

	w.raw.SetField(-2, "set")
	w.raw.SetGlobal(userdataName)

}

func (w *luaVM) Do(cache *generic.GenericSyncMap[string, any], script string, data [][]any) ([][]any, error) {
	w.input = data
	if w.input == nil {
		w.input = [][]any{}
	}

	w.tempGPtrMap = cache
	if err := lua.DoString(w.raw, script); err != nil {
		w.input = nil
		return nil, utils.ErrorfPc("%s", err.Error())
	}
	w.input = nil

	if w.output != nil && len(w.output) > 0 {
		ret := w.output
		w.output = nil
		return ret, nil
	}

	return nil, nil
}

func (w *luaVM)Extend(extendFn func(raw *lua.State) error) error {
	return extendFn(w.raw)
}
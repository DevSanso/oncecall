package vm

import (
	"fmt"
	"oncecall/utils/generic"
	"oncecall/vm/lua"
	"reflect"
	"testing"
)

func TestLuaVm(t *testing.T) {

	lua := lua.NewLuaVM()
	g := generic.NewGenericSyncMap[string, any]()
	_, err := lua.Do(g, "print('hello world')", nil)
	if err != nil {
		t.Fatal(err)
	}
}

func TestPushGoData(t *testing.T) {
	lua := lua.NewLuaVM()
	g := generic.NewGenericSyncMap[string, any]()
	_, err := lua.Do(g, `
	local function test() 
		local test1 = {}
		local test2 = 'sdfs'
		for k, val in pairs(oncecall_get_data()) do
			for index, value in ipairs(val) do
				print(index, value)
			end
		end
	end
	test()
	`, [][]any{{1234, "hello"}, {222.23, "world"}})
	if err != nil {
		t.Fatal(err)
	}
}

func TestPushGoByteData(t *testing.T) {
	lua := lua.NewLuaVM()
	g := generic.NewGenericSyncMap[string, any]()
	_, err := lua.Do(g, `
	local function test() 
		local test1 = {}
		local test2 = 'sdfs'
		for k, val in pairs(oncecall_get_data()) do
			print('test', k,val, '\n')
			for index, value in pairs(val) do
				print('ele ',index,value)
				if type(value) == "table" then
				local str = string.char(table.unpack(value))
				print(str)
				end
			end
		end
	end
	test()
	`, [][]any{{1234, []byte("hello")}, {222.23, []byte("world")}})
	if err != nil {
		t.Fatal(err)
	}
}

func TestGetGoData(t *testing.T) {
	lua := lua.NewLuaVM()
	g := generic.NewGenericSyncMap[string, any]()
	data, err := lua.Do(g, `
	
	local testing = {
		{1233, 'hello'},
		{234.12, 'world'}
	}

	local function test()
		oncecall_put_data(testing)
	end
	test()
	
	`, [][]any{{1234, "hello"}, {222, "world"}})
	if err != nil {
		t.Fatal(err)
	}

	for _, row := range data {
		for _, rowData := range row {
			fmt.Println(rowData)
		}
		fmt.Println("")
	}
}

func TestGetGoBytesData(t *testing.T) {
	lua := lua.NewLuaVM()
	g := generic.NewGenericSyncMap[string, any]()
	data, err := lua.Do(g, `
	
	local testing = {
		{1233, {10, 23, 112}}
	}

	local function test()
		oncecall_put_data(testing)
	end
	test()
	
	`, [][]any{{1234, "hello"}, {222, "world"}})
	if err != nil {
		t.Fatal(err)
	}

	conv, ok := data[0][1].([]byte)
	if !ok {
		t.Fatal("failed convert :", reflect.TypeOf(conv).Name())
	}

	for _, v := range conv {
		fmt.Println(v)
	}

	_, err = lua.Do(g, `
	
	local testing = {
		{1233, {10, 23, 112, 24444}}
	}

	local function test()
		oncecall_put_data(testing)
	end
	test()
	
	`, [][]any{{1234, "hello"}, {222, "world"}})

	if err == nil {
		t.Fatal("check failed byte range")
	} else {
		fmt.Println(err.Error())
	}
}

func TestUseCache(t *testing.T) {
	lua := lua.NewLuaVM()
	g := generic.NewGenericSyncMap[string, any]()
	_, err := lua.Do(g, `
		global_map.set("test",123)
		global_map.set("test1",123.123)
		global_map.set("test2","world")
		global_map.set("test3",{1,2,3})
		print(global_map.get("test"))
		print(global_map.get("test2222"))

	`, [][]any{{1234, "hello"}, {222, "world"}})
	if err != nil {
		t.Fatal(err)
	}

	g.Range(func(key string, value any) bool {
		fmt.Println(key, " ", value)
		return true
	})
}

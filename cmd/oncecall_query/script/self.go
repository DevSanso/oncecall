package script

import (
	"context"
	"oncecall/cmd/oncecall_query/cfg"
	"oncecall/conn"
	"oncecall/define"
	"oncecall/utils"
	"oncecall/utils/generic"
	"oncecall/vm"
	"time"

	"go.uber.org/zap"
)

type SelfScript struct{}

func NewSelfScript() *SelfScript {
	return &SelfScript{}
}

func (s *SelfScript) nextSleep(id int, conf *cfg.ScriptSelfConfig) error {
	millie := time.Now().UnixMilli()
	sec := millie / 1000
	interval := int64(conf.Interval.Sec)

	if remain := sec % interval; remain != 0 {
		sleepTime := (time.Duration(interval)*time.Second - (time.Duration(remain) * time.Second) - (time.Duration(millie%1000) * time.Millisecond)) + 10*time.Millisecond
		time.Sleep(sleepTime)

		zap.L().Debug("script.sleep", zap.String("name", conf.Name), zap.Int("id", id), zap.Bool("isSleep", true),
			zap.Float64("sleep", float64(sleepTime/time.Millisecond)*0.001))
	} else {
		zap.L().Debug("script.sleep", zap.String("name", conf.Name), zap.Int("id", id), zap.Bool("isSleep", false))
	}

	return nil
}

func (s *SelfScript) isTrigger(id int, p conn.ConnPoolInterface, m map[string]any, conf *cfg.ScriptSelfConfig) (bool, error) {
	bindData, bindDataErr := utils.ConvertMapDataBind(m, conf.GetTrigger.DynamicBind)
	if bindDataErr != nil {
		return false, utils.ErrorfPc("failed trigger [id:%d][msg:%s]", id, bindDataErr.Error())
	}

	var data [][]any = nil
	var dataErr error = nil

	if len(bindData) > 0 {
		data, dataErr = p.RunQuery(context.Background(), &conn.Args{
			Query:         conf.GetTrigger.Query,
			Args:          nil,
			IsTranscation: false,
		})
	} else {
		data, dataErr = p.RunQuery(context.Background(), &conn.Args{
			Query:         conf.GetTrigger.Query,
			Args:          [][]any{bindData},
			IsTranscation: false,
		})
	}

	if dataErr != nil {
		return false, dataErr
	}

	if data == nil || len(data) <= 0 {
		return false, nil
	} else {
		return true, nil
	}
}

func (s *SelfScript) GetData(p conn.ConnPoolInterface, conf *cfg.ScriptSelfConfig, m map[string]any) ([][]any, error) {
	var convertBindVal [][]any

	if bindData, bindErr := utils.ConvertMapDataBind(m, conf.Get.DynamicBind); bindErr != nil {
		return nil, bindErr
	} else if bindData != nil {
		convertBindVal = [][]any{bindData}
	} else {
		convertBindVal = nil
	}

	data, dataErr := p.RunQuery(context.Background(), &conn.Args{
		Query:         conf.Get.Query,
		Args:          convertBindVal,
		IsTranscation: conf.IsTranscation.Get,
	})

	if dataErr != nil {
		return nil, dataErr
	}

	return data, nil
}

func (s *SelfScript) Run(conf *cfg.ScriptSelfConfig, id int, jobCache *generic.GenericSyncMap[string, any], pMap *generic.GenericSyncMap[generic.Pair[int, bool], conn.ConnPoolInterface], vmP *generic.GenericSyncPool[vm.LuaVm]) error {
	var p conn.ConnPoolInterface
	var getPoolOk bool
	p, getPoolOk = pMap.Load(generic.Pair[int, bool]{First: id, Second: false})

	if !getPoolOk {
		return utils.ErrorfPc("get failed conn pool [%d", id)
	}
	connConfig := p.GetConfig()

	if err := s.nextSleep(id, conf); err != nil {
		return err
	}

	if isTrigger, err := s.isTrigger(id, p, connConfig.DataMap, conf); err != nil {
		return err
	} else if !isTrigger {
		return nil
	}

	var realData [][]any = nil

	if getData, getDataErr := s.GetData(p, conf, connConfig.DataMap); getDataErr != nil {
		return getDataErr
	} else if conf.Get.Script == nil {
		realData = getData
	} else {
		vm := vmP.Get()
		scriptData, scriptErr := vm.Do(jobCache, *conf.Get.Script, getData)

		if scriptErr != nil {
			return scriptErr
		}

		if scriptData == nil && len(scriptData) <= 0 {
			zap.L().Debug("script.self.vm", zap.String("name", conf.Name), zap.String("msg", "script data is none"))
		}
		
		realData = scriptData
		vmP.Put(vm)
	}

	if conf.Set.RealTime != nil && realData != nil {
		storeP, ok := pMap.Load(generic.Pair[int, bool]{First: define.ConnMapRealTimeIdx, Second: false})
		if !ok {
			return utils.ErrorfPc("not find realtime db pool")
		}
		zap.L().Debug("script.self.realtime", zap.String("name", conf.Name), zap.Any("data", realData))
		execErr := storeP.RunExecute(context.Background(), &conn.Args{
			Query:         conf.Set.RealTime.Query,
			Args:          realData,
			IsTranscation: conf.IsTranscation.Set,
		})
		if execErr != nil {
			return execErr
		}
	}

	if conf.Set.Collect != nil && realData != nil {
		storeP, ok := pMap.Load(generic.Pair[int, bool]{First: define.ConnMapCollectIdx, Second: false})
		if !ok {
			return utils.ErrorfPc("not find collect db pool")
		}
		zap.L().Debug("script.self.collect", zap.String("name", conf.Name), zap.Any("data", realData))
		execErr := storeP.RunExecute(context.Background(), &conn.Args{
			Query:         conf.Set.Collect.Query,
			Args:          realData,
			IsTranscation: conf.IsTranscation.Set,
		})
		if execErr != nil {
			return execErr
		}
	}

	return nil
}

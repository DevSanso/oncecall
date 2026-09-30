package utils

import "sync"

type PoolMetricData struct {
	use   int
	alloc int

	top10InputDataLang  [10]int
	top10OutputDataLang [10]int
}
type connPoolMetric struct {
	rw sync.RWMutex

	stat PoolMetricData
}

func (c *connPoolMetric) Current() PoolMetricData {
	c.rw.RLock()
	defer c.rw.RUnlock()

	return c.stat
}

func (c *connPoolMetric) ConnPoolStatUpdate(addUse, addAlloc, inputDataLen, outputDataLen int) {
	c.rw.Lock()
	defer c.rw.Unlock()

	c.stat.use += addUse
	c.stat.alloc += addAlloc

	var bit = 0
	for i := 0; i < len(c.stat.top10InputDataLang); i++ {
		if (bit & 0x11) == 0x11 {
			break
		}

		if c.stat.top10InputDataLang[i] < inputDataLen {
			c.stat.top10InputDataLang[i] = inputDataLen
		} else {
			bit = bit | 0x1
		}

		if c.stat.top10OutputDataLang[i] < outputDataLen {
			c.stat.top10OutputDataLang[i] = outputDataLen
		} else {
			bit = bit | 0x10
		}
	}
}

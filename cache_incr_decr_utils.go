package cache

import (
	"errors"
	"fmt"
	"math"
	"strconv"
)

func toInt64(v interface{}) (int64, error) {
	switch v := v.(type) {
	case int:
		return int64(v), nil
	case int8:
		return int64(v), nil
	case int16:
		return int64(v), nil
	case int32:
		return int64(v), nil
	case int64:
		return v, nil
	case uint8:
		return int64(v), nil
	case uint16:
		return int64(v), nil
	case uint32:
		return int64(v), nil
	case uint:
		if uint64(v) > math.MaxInt64 {
			return 0, fmt.Errorf("value %d overflows int64", v)
		}
		return int64(v), nil
	case uint64:
		if v > math.MaxInt64 {
			return 0, fmt.Errorf("value %d overflows int64", v)
		}
		return int64(v), nil
	case string:
		return strconv.ParseInt(v, 10, 64)
	default:
		return 0, fmt.Errorf("unsupported type %T", v)
	}
}

func calculateIncrOrDecr(input interface{}, incrOrDecr int) (int64, error) {
	isDecr := incrOrDecr < 0 // check the operation
	if isDecr {
		incrOrDecr = -incrOrDecr // keep it always positive
	}

	switch val := input.(type) {
	case int, int8, int16, int32, int64, uint, uint8, uint16, uint32, uint64:
		curr, err := toInt64(val)
		if err != nil {
			return curr, err
		}

		if (curr > math.MaxInt64-int64(incrOrDecr)) || (curr < math.MinInt64+int64(incrOrDecr)) {
			return curr, errors.New("value would overflow int64")
		}

		if isDecr {
			return curr - int64(incrOrDecr), nil
		}

		return curr + int64(incrOrDecr), nil

	default:
		return 0, errors.New("value is not (u)int (u)int8 (u)int16 (u)int32 (u)int64")
	}
}

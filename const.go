package aac

import "math"

const (
	SCE = iota
	CPE
	CCE
	LFE
	DSE
	PCE
	FIL
	END
)

const (
	FILL_DATA = iota + 1
	DATA_ELEMENT
	DYNAMIC_RANGE = 11
	SBR_DATA      = 12
	SBR_DATA_CRC  = 13
)

const (
	ONLY_LONG_SEQUENCE = iota
	LONG_START_SEQUENCE
	EIGHT_SHORT_SEQUENCE
	LONG_STOP_SEQUENCE
)

const (
	ZERO_HCB       = 0
	FIRST_PAIR_HCB = 5
	ESC_HCB        = 11
	QUAD_LEN       = 4
	PAIR_LEN       = 2
	LEN_TAG        = 4
	NOISE_HCB      = 13
	INTENSITY_HCB2 = 14
	INTENSITY_HCB  = 15
)

const (
	LAST_CB_IDX = 11
)

const (
	MAX_SFB       = 51
	MAX_CHANNELS  = 64
	TNS_MAX_ORDER = 20
)

const (
	FRAC_BITS = iota + 31
	FRAC_SIZE
	FRAC_MUL = 1 << (FRAC_SIZE - FRAC_BITS)
)

const (
	COEF_BITS = 28
)

const FLOAT_SCALE = 1.0 / (1 << 15)

func MUL_C(a, b float64) float64 {
	return float64(int64(a)*int64(b) + (1<<(COEF_BITS-1))>>COEF_BITS)
}

func CONV(a, b uint) uint {
	return (a << 1) | (b & 0x1)
}

func SAT_SHIFT_MASK(E uint) uint {
	return ^uint(0) << (31 - E)
}

func SAT_SHIFT(V int32, E uint, M uint) int32 {
	if (((V >> (E + 1)) ^ V) & int32(M)) != 0 {
		if V < 0 {
			return math.MinInt32 // Equivalent to 0x80000000 in C
		} else {
			return math.MaxInt32 // Equivalent to 0x7FFFFFFF in C
		}
	} else {
		return int32(uint32(V) << E)
	}
}

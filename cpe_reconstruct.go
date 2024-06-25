package aac

import (
	"math"
)

//Code from FAAD2 is copyright (c) Nero AG, www.nero.com

func reconstruct_channel_pair(d *Decoder, ics [2]*ic_stream, spec_data [2][1024]int16) {
	var spec_coef1, spec_coef2 [1024]float64
	quant_to_spec(ics[0], spec_data[0], &spec_coef1)
	quant_to_spec(ics[0], spec_data[0], &spec_coef2)

	//apply_scale_factors(d.reader, ics[0], &spec_coef1)
	//apply_scale_factors(d.reader, ics[1], &spec_coef2)
	pns_decode(d.reader, ics[0], ics[1], &spec_coef1, &spec_coef2, ics[0].ms_mask_present != 0)
	ms_decode(d.reader, ics[0], ics[1], &spec_coef1, &spec_coef2)
	is_decode(d.reader, ics[0], ics[1], &spec_coef1, &spec_coef2)

	tns_decode_frame(ics[0], d.reader.sf_index, spec_coef1, d.reader.frame_length)
	tns_decode_frame(ics[1], d.reader.sf_index, spec_coef2, d.reader.frame_length)

	d.time_out[0], d.time_out[1] = make([]float64, d.reader.frame_length), make([]float64, d.reader.frame_length)
	d.fb_intermed[0], d.fb_intermed[1] = make([]float64, d.reader.frame_length), make([]float64, d.reader.frame_length)
	ifilter_bank(d.fb, ics[0].window_sequence, ics[0].window_shape, d.prev_window_shape[0], spec_coef1[:], d.time_out[0], d.fb_intermed[0], d.reader.frame_length)
	ifilter_bank(d.fb, ics[1].window_sequence, ics[1].window_shape, d.prev_window_shape[1], spec_coef2[:], d.time_out[1], d.fb_intermed[1], d.reader.frame_length)

	d.prev_window_shape[0] = ics[0].window_shape
	d.prev_window_shape[1] = ics[1].window_shape
}

func inverse_quantization(spec_coeff *[1024]float64, spec_data [1024]int16) {
	for i := 0; i < 1024; i++ {
		spec_coeff[i] = math.Pow(float64(abs(spec_data[i])), (4 / 3))
		if spec_data[i] < 0 {
			spec_coeff[i] = -spec_coeff[i]
		}
	}
}

func apply_scale_factors(rd Reader, ics *ic_stream, spec_coeff *[1024]float64) {
	nshort := rd.frame_length / 8
	var groups uint
	for g := uint(0); g < ics.num_window_groups-1; g++ {
		var k uint
		for sfb := uint(0); sfb < ics.max_sfb-1; sfb++ {
			top := ics.sect_sfb_offset[g][sfb+1]
			var scale float64
			if ics.scale_factors[g][sfb] < 0 || ics.scale_factors[g][sfb] > 255 {
				scale = 1
			} else {
				exponent := (float64(ics.scale_factors[g][sfb]) - 100) / 4.0
				scale = math.Pow(2, exponent)
			}
			for k < top {
				spec_coeff[k+(groups*nshort)+0] *= scale
				spec_coeff[k+(groups*nshort)+1] *= scale
				spec_coeff[k+(groups*nshort)+2] *= scale
				spec_coeff[k+(groups*nshort)+3] *= scale
				k += 4
			}
		}
		groups += ics.window_group_length[g]
	}
}

func pns_decode(rd Reader, ics1, ics2 *ic_stream, spec_coeff1, spec_coeff2 *[1024]float64, channel_pair bool) {
	var group uint
	nshort := rd.frame_length / 8
	for g := uint(0); g < ics1.num_window_groups-1; g++ {
		for b := uint(0); b < ics1.window_group_length[g]-1; b++ {
			for sfb := uint(0); sfb < ics1.max_sfb-1; sfb++ {
				if ics1.sfb_cb[group][sfb] == NOISE_HCB {
					offset := ics1.swb_offset[sfb]
					size := ics1.swb_offset[sfb+1] - offset
					generate_random_vector(spec_coeff1[(group*nshort)+offset:], ics1.scale_factors[g][sfb], size)
				}
				if channel_pair && ics2.sfb_cb[g][sfb] == NOISE_HCB {
					if ics1.ms_mask_present == 1 && ics1.ms_used[g][sfb] == 1 || ics1.ms_mask_present == 2 {
						offset := ics2.swb_offset[sfb]
						size := ics2.swb_offset[sfb+1] - offset
						for c := uint(0); c < size; c++ {
							spec_coeff2[(group*nshort)+offset+c] = spec_coeff1[(group*nshort)+offset+c]
						}
					} else {
						offset := ics2.swb_offset[sfb]
						size := ics2.swb_offset[sfb+1] - offset
						generate_random_vector(spec_coeff2[(group*nshort)+offset:], ics2.scale_factors[g][sfb], size)
					}
				}
			}
		}
		group++
	}
}

func generate_random_vector(spec_coeff []float64, sf, size uint) {
	var energy float64
	var scale = 1 / float64(size)
	for i := uint(0); i < size; i++ {
		tmp := scale * float64(random_int())
		spec_coeff[i] = float64(tmp)
		energy += tmp * tmp
	}
	exponent := 0.25 * float64(sf)
	scale = math.Pow(2, exponent) / math.Sqrt(energy)
	for i := uint(0); i < size; i++ {
		spec_coeff[i] *= scale
	}
}

func quant_to_spec(ics *ic_stream, quant_data [1024]int16, spec_data *[1024]float64) {
	var pow2_table = []float64{
		1.0,
		1.1892071150027210667174999705605,
		1.4142135623730950488016887242097,
		1.6817928305074290860622509524664,
	}
	tab := iq_table

	var width, k, gindex uint
	var sat_shift_mask int

	for g := uint(0); g < ics.num_window_groups; g++ {
		var j uint
		var gincrease uint
		var win_inc = ics.swb_offset[ics.num_swb]

		for sfb := uint(0); sfb < ics.num_swb; sfb++ {
			var exp, frac int
			var wa = gindex + j
			var scale_factor = ics.scale_factors[g][sfb]

			width = ics.swb_offset[sfb+1] - ics.swb_offset[sfb]

			if is_intensity(ics, g, sfb) || is_noise(ics, g, sfb) {
				scale_factor = 0
			}

			exp = int(scale_factor) >> 2
			frac = int(scale_factor) & 3

			if exp > 0 {
				sat_shift_mask = int(SAT_SHIFT_MASK(uint(exp)))
			}

			for win := uint(0); win < ics.window_group_length[g]; win++ {
				for bin := uint(0); bin < width; bin += 4 {
					wb := wa + bin
					iq0 := iquant(quant_data[k+0], tab[:])
					iq1 := iquant(quant_data[k+1], tab[:])
					iq2 := iquant(quant_data[k+2], tab[:])
					iq3 := iquant(quant_data[k+3], tab[:])

					if exp == -32 {
						spec_data[wb+0] = 0
						spec_data[wb+1] = 0
						spec_data[wb+2] = 0
						spec_data[wb+3] = 0
					} else if exp <= 0 {
						spec_data[wb+0] = float64(int16(iq0) >> -exp)
						spec_data[wb+1] = float64(int16(iq1) >> -exp)
						spec_data[wb+2] = float64(int16(iq2) >> -exp)
						spec_data[wb+3] = float64(int16(iq3) >> -exp)
					} else {
						spec_data[wb+0] = float64(SAT_SHIFT(int32(iq0), uint(exp), uint(sat_shift_mask)))
						spec_data[wb+1] = float64(SAT_SHIFT(int32(iq1), uint(exp), uint(sat_shift_mask)))
						spec_data[wb+2] = float64(SAT_SHIFT(int32(iq2), uint(exp), uint(sat_shift_mask)))
						spec_data[wb+3] = float64(SAT_SHIFT(int32(iq3), uint(exp), uint(sat_shift_mask)))
					}

					if frac != 0 {
						spec_data[wb+0] = MUL_C(spec_data[wb+0], pow2_table[frac])
						spec_data[wb+1] = MUL_C(spec_data[wb+1], pow2_table[frac])
						spec_data[wb+2] = MUL_C(spec_data[wb+2], pow2_table[frac])
						spec_data[wb+3] = MUL_C(spec_data[wb+3], pow2_table[frac])
					}

					gincrease += 4
					k += 4
				}
				wa += win_inc
			}
			j += width
		}
		gindex += gincrease
	}
}

func iquant(q int16, tab []float64) float64 {
	if q < 0 {
		if -q < IQ_TABLE_SIZE {
			return -tab[-q]
		}

		return 0
	} else {
		if q < IQ_TABLE_SIZE {
			return tab[q]
		}

		return 0
	}
}

func ms_decode(rd Reader, ics1, ics2 *ic_stream, spec_coeff1, spec_coeff2 *[1024]float64) {
	var group uint
	nshort := rd.frame_length / 8
	if ics1.ms_mask_present != 0 {
		for g := uint(0); g < ics1.num_window_groups-1; g++ {
			for b := uint(0); b < ics1.window_group_length[g]-1; b++ {
				for sfb := uint(0); sfb < ics1.max_sfb-1; sfb++ {
					if (ics1.ms_used[g][sfb] != 0 || ics1.ms_mask_present == 2) &&
						ics1.sfb_cb[group][sfb] != NOISE_HCB &&
						ics1.sfb_cb[group][sfb] != INTENSITY_HCB &&
						ics1.sfb_cb[group][sfb] != INTENSITY_HCB2 {
						for i := ics1.swb_offset[sfb]; i < ics1.swb_offset[sfb+1]; i++ {
							k := (group * nshort) + i
							tmp := spec_coeff1[k] - spec_coeff2[k]
							spec_coeff1[k] += spec_coeff2[k]
							spec_coeff2[k] = tmp
						}
					}
				}
				group++
			}
		}
	}
}

func is_decode(rd Reader, ics1, ics2 *ic_stream, spec_coeff1, spec_coeff2 *[1024]float64) {
	nshort := rd.frame_length / 8
	var group uint
	for g := uint(0); g < ics2.num_window_groups; g++ {
		for b := uint(0); b < ics2.window_group_length[g]; b++ {
			for sfb := uint(0); sfb < ics2.max_sfb; sfb++ {
				if ics2.sfb_cb[group][sfb] == INTENSITY_HCB || ics2.sfb_cb[group][sfb] == INTENSITY_HCB2 {
					exponent := 0.25 * float64(ics2.scale_factors[g][sfb])
					scale := math.Pow(0.5, exponent)
					for i := ics2.swb_offset[sfb]; i < ics2.swb_offset[sfb+1]; i++ {
						spec_coeff2[(group*nshort)+i] = spec_coeff1[(group*nshort)+i] * scale
					}
				}
			}
			group++
		}
	}
}

var r1, r2 uint = 0x2bb431ea, 0x206155b7

func random_int() uint {
	var t1, t2 uint

	var t3 = t1 - r2
	var t4 = t2 - r2

	t1 &= 0xF5
	t2 >>= 25
	t1 = parity[t1]
	t2 &= 0x63
	t1 <<= 31
	t2 = parity[t2]
	r1 = (t3 >> 1) | t1
	r2 = (t4 + t4) | t2

	return r1 ^ r2
}

var parity = [256]uint{
	0, 1, 1, 0, 1, 0, 0, 1, 1, 0, 0, 1, 0, 1, 1, 0, 1, 0, 0, 1, 0, 1, 1, 0, 0, 1, 1, 0, 1, 0, 0, 1,
	1, 0, 0, 1, 0, 1, 1, 0, 0, 1, 1, 0, 1, 0, 0, 1, 0, 1, 1, 0, 1, 0, 0, 1, 1, 0, 0, 1, 0, 1, 1, 0,
	1, 0, 0, 1, 0, 1, 1, 0, 0, 1, 1, 0, 1, 0, 0, 1, 0, 1, 1, 0, 1, 0, 0, 1, 1, 0, 0, 1, 0, 1, 1, 0,
	0, 1, 1, 0, 1, 0, 0, 1, 1, 0, 0, 1, 0, 1, 1, 0, 1, 0, 0, 1, 0, 1, 1, 0, 0, 1, 1, 0, 1, 0, 0, 1,
	1, 0, 0, 1, 0, 1, 1, 0, 0, 1, 1, 0, 1, 0, 0, 1, 0, 1, 1, 0, 1, 0, 0, 1, 1, 0, 0, 1, 0, 1, 1, 0,
	0, 1, 1, 0, 1, 0, 0, 1, 1, 0, 0, 1, 0, 1, 1, 0, 1, 0, 0, 1, 0, 1, 1, 0, 0, 1, 1, 0, 1, 0, 0, 1,
	0, 1, 1, 0, 1, 0, 0, 1, 1, 0, 0, 1, 0, 1, 1, 0, 1, 0, 0, 1, 0, 1, 1, 0, 0, 1, 1, 0, 1, 0, 0, 1,
	1, 0, 0, 1, 0, 1, 1, 0, 0, 1, 1, 0, 1, 0, 0, 1, 0, 1, 1, 0, 1, 0, 0, 1, 1, 0, 0, 1, 0, 1, 1, 0,
}

func abs(i int16) int16 {
	if i < 0 {
		return -i
	}
	return i
}

func is_intensity(ics *ic_stream, group, sfb uint) bool {
	return ics.sfb_cb[group][sfb] == INTENSITY_HCB || ics.sfb_cb[group][sfb] == INTENSITY_HCB2
}

func is_noise(ics *ic_stream, group, sfb uint) bool {
	return ics.sfb_cb[group][sfb] == NOISE_HCB
}

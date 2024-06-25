package aac

//Code from FAAD2 is copyright (c) Nero AG, www.nero.com

type tns_info struct {
	n_filter [8]uint
	coef_res [8]uint
	length,
	order,
	direction,
	coef_compress [8][4]uint
	coef [8][4][32]uint
}

var tns_coef_0_3 = []float64{
	(0.0), (0.4338837391), (0.7818314825), (0.9749279122),
	(-0.9848077530), (-0.8660254038), (-0.6427876097), (-0.3420201433),
	(-0.4338837391), (-0.7818314825), (-0.9749279122), (-0.9749279122),
	(-0.9848077530), (-0.8660254038), (-0.6427876097), (-0.3420201433),
}
var tns_coef_0_4 = []float64{
	(0.0), (0.2079116908), (0.4067366431), (0.5877852523),
	(0.7431448255), (0.8660254038), (0.9510565163), (0.9945218954),
	(-0.9957341763), (-0.9618256432), (-0.8951632914), (-0.7980172273),
	(-0.6736956436), (-0.5264321629), (-0.3612416662), (-0.1837495178),
}
var tns_coef_1_3 = []float64{
	(0.0), (0.4338837391), (-0.6427876097), (-0.3420201433),
	(0.9749279122), (0.7818314825), (-0.6427876097), (-0.3420201433),
	(-0.4338837391), (-0.7818314825), (-0.6427876097), (-0.3420201433),
	(-0.7818314825), (-0.4338837391), (-0.6427876097), (-0.3420201433),
}
var tns_coef_1_4 = []float64{
	(0.0), (0.2079116908), (0.4067366431), (0.5877852523),
	(-0.6736956436), (-0.5264321629), (-0.3612416662), (-0.1837495178),
	(0.9945218954), (0.9510565163), (0.8660254038), (0.7431448255),
	(-0.6736956436), (-0.5264321629), (-0.3612416662), (-0.1837495178),
}

var all_tns_coefs = [][]float64{tns_coef_0_3, tns_coef_0_4, tns_coef_1_3, tns_coef_1_4}

func tns_data(rd Reader, ics *ic_stream) {
	var (
		coef_bits     uint
		n_filter_bits = 2
		length_bits   = 6
		order_bits    = 5
	)

	if ics.window_sequence == EIGHT_SHORT_SEQUENCE {
		n_filter_bits = 1
		length_bits = 4
		order_bits = 3
	}

	for w := uint(0); w < ics.num_windows; w++ {
		const start_coef_bits = 3
		ics.tns.n_filter[w], _ = rd.Read(n_filter_bits)
		if ics.tns.n_filter[w] != 0 {
			ics.tns.coef_res[w], _ = rd.Read(1)
		}

		for filter := uint(0); filter < ics.tns.n_filter[w]; filter++ {
			ics.tns.length[w][filter], _ = rd.Read(length_bits)
			ics.tns.order[w][filter], _ = rd.Read(order_bits)

			if ics.tns.order[w][filter] != 0 {
				ics.tns.direction[w][filter], _ = rd.Read(1)
				ics.tns.coef_compress[w][filter], _ = rd.Read(1)

				coef_bits = start_coef_bits - ics.tns.coef_compress[w][filter]
				for i := uint(0); i < ics.tns.order[w][filter]; i++ {
					ics.tns.coef[w][filter][i], _ = rd.Read(int(coef_bits))
				}
			}
		}
	}
}

func tns_decode_frame(ics *ic_stream, sr_index uint, spec_coef [1024]float64, frame_length uint) {
	var (
		tns_order               uint
		bottom, top, start, end uint
		nshort                  = frame_length / 8
		exp                     uint

		size int
		inc  int

		lpc [TNS_MAX_ORDER + 1]float64
	)
	if !ics.tns_data_present {
		return
	}

	for w := uint(0); w < ics.num_windows; w++ {
		bottom = ics.num_swb

		for f := uint(0); f < ics.tns.n_filter[w]; f++ {
			top = bottom
			bottom = max(top-ics.tns.length[w][f], 0)
			tns_order = min(ics.tns.order[w][f], TNS_MAX_ORDER)

			if tns_order == 0 {
				continue
			}

			exp = tns_decode_coef(tns_order, ics.tns.coef_res[w]+3, ics.tns.coef_compress[w][f], ics.tns.coef[w][f][:], lpc[:])

			start = min(bottom, max_tns_sfb(sr_index, ics.window_sequence == EIGHT_SHORT_SEQUENCE))
			start = min(start, ics.max_sfb)
			start = min(ics.swb_offset[start], ics.swb_offset_max)

			end = min(top, max_tns_sfb(sr_index, ics.window_sequence == EIGHT_SHORT_SEQUENCE))
			end = min(end, ics.max_sfb)
			end = min(ics.swb_offset[end], ics.swb_offset_max)

			size = int(end - start)

			if size <= 0 {
				continue
			}

			if ics.tns.direction[w][f] != 0 {
				inc = -1
				start = end - 1
			} else {
				inc = 1
			}

			tns_ar_filter(spec_coef, int((w*nshort)+start), size, inc, lpc[:], tns_order, exp)
		}
	}
}

func tns_ar_filter(spec_coef [1024]float64, spectrum_index int, size, inc int, lpc []float64, order, exp uint) {
	var (
		state       [2 * TNS_MAX_ORDER]float64
		state_index int
	)

	spectrum := spec_coef[spectrum_index:]
	for i := 0; i < size; i++ {
		y := 0.0
		for j := 0; j < int(order); j++ {
			y += MUL_C(state[state_index+j], lpc[j+1])
		}
		y = spectrum[0] - y

		state_index--
		if state_index < 0 {
			state_index = int(order) - 1
		}
		state[state_index] = y
		state[state_index+int(order)] = y

		spectrum[0] = y
		spectrum_index += inc

		spectrum = spec_coef[spectrum_index-inc:]
	}
}

func tns_decode_coef(order, coef_res_bits, coef_compress uint, coef []uint, a []float64) uint {
	var (
		tmp2, b     [TNS_MAX_ORDER + 1]float64
		table_index = 2*booluint(coef_compress != 0) + booluint(coef_res_bits != 3)
		tns_coef    = all_tns_coefs[table_index]
		exp         uint
	)

	for i := uint(0); i < order; i++ {
		tmp2[i] = tns_coef[coef[i]]
	}

	a[0] = 1.0

	for m := uint(1); m <= order; m++ {
		a[m] = tmp2[m-1]
		for i := uint(1); i < m; i++ {
			b[i] = a[i] + MUL_C(a[m], a[m-i])
		}
		for i := uint(1); i < m; i++ {
			a[i] = b[i]
		}
	}

	return exp
}

func booluint(b bool) uint {
	if b {
		return 1
	}
	return 0
}

func max_tns_sfb(sr_index uint, is_short bool) uint {
	var tns_sbf_max = [][4]uint{
		{31, 9, 28, 7},  /* 96000 */
		{31, 9, 28, 7},  /* 88200 */
		{34, 10, 27, 7}, /* 64000 */
		{40, 14, 26, 6}, /* 48000 */
		{42, 14, 26, 6}, /* 44100 */
		{51, 14, 26, 6}, /* 32000 */
		{46, 14, 29, 7}, /* 24000 */
		{46, 14, 29, 7}, /* 22050 */
		{42, 14, 23, 8}, /* 16000 */
		{42, 14, 23, 8}, /* 12000 */
		{42, 14, 23, 8}, /* 11025 */
		{39, 14, 19, 7}, /*  8000 */
		{39, 14, 19, 7}, /*  7350 */
		{0, 0, 0, 0},
		{0, 0, 0, 0},
		{0, 0, 0, 0},
	}
	var i uint
	if is_short {
		i++
	}

	return tns_sbf_max[sr_index][i]
}

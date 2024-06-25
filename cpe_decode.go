package aac

//Code from FAAD2 is copyright (c) Nero AG, www.nero.com

func bit_set(a, b uint) uint {
	return a & (1 << b)
}

type element struct {
	instance_tag  uint
	common_window bool
}

type ic_stream struct {
	element               element
	window_sequence       uint
	window_shape          uint
	scale_factor_grouping uint
	max_sfb               uint

	num_sec             [8]uint
	num_swb             uint
	num_windows         uint
	num_window_groups   uint
	window_group_length [8]uint

	ms_used, scale_factors [8][51]uint

	swb_offset     [52]uint
	swb_offset_max uint

	sect_sfb_offset,
	sect_cb, sect_start, sect_end, sfb_cb [8][15 * 8]uint

	predictor_data_present bool

	global_gain uint

	noise_used, is_used bool

	ms_mask_present  uint
	tns_data_present bool

	pul pulse_info
	tns tns_info
}

type pulse_info struct {
	number_pulse, pulse_start_sfb uint
	pulse_offset, pulse_amp       [4]uint
}

func decode_cpe(d *Decoder) {
	var element element
	element.instance_tag, _ = d.reader.Read(LEN_TAG)
	element.common_window, _ = d.reader.ReadFlag()
	var (
		spec_data1, spec_data2 [1024]int16
		ics1, ics2             ic_stream
	)
	ics1.element = element
	ics2.element = element

	if element.common_window {
		ics_info(d.reader, &ics1)
		ics1.ms_mask_present, _ = d.reader.Read(2)
		if ics1.ms_mask_present == 1 {
			for g := uint(0); g < ics1.num_window_groups; g++ {
				for sfb := uint(0); sfb < ics1.max_sfb; sfb++ {
					ics1.ms_used[g][sfb], _ = d.reader.Read(1)
				}
			}
		}
		ics2 = ics1
	}
	individual_channel_stream(d.reader, &ics1, &spec_data1)
	individual_channel_stream(d.reader, &ics2, &spec_data2)

	reconstruct_channel_pair(d, [2]*ic_stream{&ics1, &ics2}, [2][1024]int16{spec_data1, spec_data2})

}

func ics_info(rd Reader, ics *ic_stream) {
	rd.Skip(1) // reserved
	ics.window_sequence, _ = rd.Read(2)
	ics.window_shape, _ = rd.Read(1)

	if ics.window_sequence == EIGHT_SHORT_SEQUENCE {
		ics.max_sfb, _ = rd.Read(4)
		ics.scale_factor_grouping, _ = rd.Read(7)
	} else {
		ics.max_sfb, _ = rd.Read(6)
	}
	window_grouping_info(rd, ics)

	if ics.window_sequence != EIGHT_SHORT_SEQUENCE {
		ics.predictor_data_present, _ = rd.ReadFlag()
	}
}

func window_grouping_info(rd Reader, ics *ic_stream) {
	switch ics.window_sequence {
	case ONLY_LONG_SEQUENCE, LONG_START_SEQUENCE, LONG_STOP_SEQUENCE:
		ics.num_windows = 1
		ics.num_window_groups = 1
		ics.window_group_length[ics.num_window_groups-1] = 1

		if rd.frame_length == 1024 {
			ics.num_swb = num_swb_1024_window[rd.sf_index]
		} else {
			ics.num_swb = num_swb_960_window[rd.sf_index]
		}

		for i := uint(0); i < ics.num_swb; i++ {
			ics.sect_sfb_offset[0][i] = swb_offset_1024_window[rd.sf_index][i]
			ics.swb_offset[i] = swb_offset_1024_window[rd.sf_index][i]
		}
		ics.sect_sfb_offset[0][ics.num_swb] = rd.frame_length
		ics.swb_offset[ics.num_swb] = rd.frame_length
		ics.swb_offset_max = rd.frame_length
	case EIGHT_SHORT_SEQUENCE:
		ics.num_windows = 8
		ics.num_window_groups = 1
		ics.window_group_length[ics.num_window_groups-1] = 1
		ics.num_swb = num_swb_128_window[rd.sf_index]

		for i := uint(0); i < ics.num_swb; i++ {
			ics.swb_offset[i] = swb_offset_128_window[rd.sf_index][i]
		}
		ics.swb_offset[ics.num_swb] = rd.frame_length / 8
		ics.swb_offset_max = rd.frame_length / 8
		for i := uint(0); i < ics.num_windows-1; i++ {
			if bit_set(ics.scale_factor_grouping, 6-i) == 0 {
				ics.num_window_groups += 1
				ics.window_group_length[ics.num_window_groups-1] = 1
			} else {
				ics.window_group_length[ics.num_window_groups-1] += 1
			}
		}

		for g := uint(0); g < ics.num_window_groups; g++ {
			var width, sect_sfb, offset uint

			for i := uint(0); i < ics.num_swb; i++ {
				if i+1 == ics.num_swb {
					width = (rd.frame_length / 8) - swb_offset_128_window[rd.sf_index][i]
				} else {
					width = swb_offset_128_window[rd.sf_index][i+1] - swb_offset_128_window[rd.sf_index][i]
				}
				width *= ics.window_group_length[g]
				sect_sfb++
				ics.sect_sfb_offset[g][sect_sfb] = offset
				offset += width
			}
			ics.sect_sfb_offset[g][sect_sfb] = offset
		}
	}
}

func individual_channel_stream(rd Reader, ics *ic_stream, spec_data *[1024]int16) {
	ics.global_gain, _ = rd.Read(8)
	if !ics.element.common_window {
		ics_info(rd, ics)
	}
	section_data(rd, ics)
	decode_scale_factors(rd, ics)
	pulse_data_present, _ := rd.ReadFlag()
	if pulse_data_present {
		pulse_data(rd, ics)
	}
	ics.tns_data_present, _ = rd.ReadFlag()
	if ics.tns_data_present {
		tns_data(rd, ics)
	}
	rd.ReadFlag() // gain_control_data_present
	spectral_data(rd, ics, spec_data)
}

func pulse_data(rd Reader, ics *ic_stream) {
	ics.pul.number_pulse, _ = rd.Read(2)
	ics.pul.pulse_start_sfb, _ = rd.Read(6)
	for i := uint(0); i < ics.pul.number_pulse+1; i++ {
		ics.pul.pulse_offset[i], _ = rd.Read(5)
		ics.pul.pulse_amp[i], _ = rd.Read(4)
	}
}

func spectral_data(rd Reader, ics *ic_stream, spec_data *[1024]int16) {
	var (
		p      uint
		groups uint = 0
		nshort      = rd.frame_length / 8
	)
	for g := uint(0); g < ics.num_window_groups; g++ {
		p = nshort * groups
		for i := uint(0); i < ics.num_sec[g]; i++ {
			section_codebook := ics.sect_cb[g][i]
			var increment uint = 4
			if section_codebook >= FIRST_PAIR_HCB {
				increment = 2
			}
			switch section_codebook {
			case ZERO_HCB, NOISE_HCB, INTENSITY_HCB, INTENSITY_HCB2:
				p += ics.sect_sfb_offset[g][ics.sect_end[g][i]] - ics.sect_sfb_offset[g][ics.sect_start[g][i]]
			default:
				for k := ics.sect_sfb_offset[g][ics.sect_start[g][i]]; k < ics.sect_sfb_offset[g][ics.sect_end[g][i]]; k += increment {
					huffman_spectral_data(section_codebook, rd, spec_data[p:])

					p += increment
				}
			}
		}
		groups += ics.window_group_length[g]
	}
}

func huffman_scale_factor(rd Reader) uint {
	var (
		offset uint
	)
	for hcb_sf[offset][1] != 0 {
		b, _ := rd.Read(1)
		offset += hcb_sf[offset][b]
	}

	return hcb_sf[offset][0]
}

func decode_scale_factors(rd Reader, ics *ic_stream) {
	var (
		t              uint
		noise_pcm_flag = true
		scale_factor   = ics.global_gain
		is_position    uint
		noise_energy   = ics.global_gain - 90
	)
	for g := uint(0); g < ics.num_window_groups; g++ {
		for sfb := uint(0); sfb < ics.max_sfb; sfb++ {
			switch ics.sfb_cb[g][sfb] {
			case ZERO_HCB:
				ics.scale_factors[g][sfb] = 0
			case INTENSITY_HCB, INTENSITY_HCB2:
				t = huffman_scale_factor(rd)
				is_position += (t - 60)
				ics.scale_factors[g][sfb] = is_position
			case NOISE_HCB:
				if noise_pcm_flag {
					noise_pcm_flag = false
					t, _ = rd.Read(9)
				} else {
					t = huffman_scale_factor(rd) - 60
				}
				noise_energy += t
				ics.scale_factors[g][sfb] = noise_energy
			default:
				t = huffman_scale_factor(rd)
				scale_factor += (t - 60)
				ics.scale_factors[g][sfb] = scale_factor
			}
		}
	}
}

func section_data(rd Reader, ics *ic_stream) {
	var (
		g             uint
		section_bits       = 5
		section_limit uint = MAX_SFB
	)
	if ics.window_sequence == EIGHT_SHORT_SEQUENCE {
		section_bits = 3
		section_limit = 8 * 15
	}
	section_escape_value := uint(1<<section_bits - 1)

	for ; g < ics.num_window_groups; g++ {
		var (
			k, i uint
		)
		for k < ics.max_sfb {
			var (
				section_length uint
			)

			if i >= section_limit {
				return
			}
			const section_codebook_bits = 4
			ics.sect_cb[g][i], _ = rd.Read(section_codebook_bits)
			if ics.sect_cb[g][i] == NOISE_HCB {
				ics.noise_used = true
			}
			if ics.sect_cb[g][i] == INTENSITY_HCB || ics.sect_cb[g][i] == INTENSITY_HCB2 {
				ics.is_used = true
			}
			section_length_increment, _ := rd.Read(section_bits)
			for section_length_increment == section_escape_value {
				section_length += section_length_increment
				section_length_increment, _ = rd.Read(section_bits)
			}
			section_length += section_length_increment
			ics.sect_start[g][i] = k
			ics.sect_end[g][i] = k + section_length
			for sfb := k; sfb < k+section_length; sfb++ {
				ics.sfb_cb[g][sfb] = ics.sect_cb[g][i]
			}
			k += section_length
			i++
		}
		ics.num_sec[g] = i
	}
}

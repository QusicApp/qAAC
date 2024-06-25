package aac

//Code from FAAD2 is copyright (c) Nero AG, www.nero.com

type fb_info struct {
	long_window, short_window [2][]float64
	mdct256, mdct2048         *mdct_info
}

func imdct_long(fb *fb_info, in_data, out_data []float64) {
	imdct(fb.mdct2048, in_data, out_data)
}

func filter_bank_init(frame_len uint) *fb_info {
	var nshort = frame_len / 8

	fb := new(fb_info)
	fb.mdct256 = mdct_init(2 * nshort)
	fb.mdct2048 = mdct_init(2 * frame_len)

	fb.long_window[0] = sine_long_1024
	fb.short_window[0] = sine_short_128
	fb.long_window[1] = kbd_long_1024
	fb.short_window[1] = kbd_short_128

	return fb
}

func ifilter_bank(fb *fb_info, window_sequence, window_shape, window_shape_prev uint, freq_in, time_out, overlap []float64, frame_len uint) {
	var i uint
	var transf_buf [2 * 1024]float64

	var (
		window_long,
		window_long_prev,
		window_short,
		window_short_prev []float64
	)

	var (
		nlong  = frame_len
		nshort = frame_len / 8
		trans  = nshort / 2

		nflat_ls = (nlong - nshort) / 2
	)
	window_long = fb.long_window[window_shape]
	window_long_prev = fb.long_window[window_shape_prev]
	window_short = fb.short_window[window_shape]
	window_short_prev = fb.short_window[window_shape_prev]

	switch window_sequence {
	case ONLY_LONG_SEQUENCE:
		imdct_long(fb, freq_in, transf_buf[:])

		for i = 0; i < nlong; i += 4 {
			time_out[i] = overlap[i] + transf_buf[i]*window_long_prev[i]
			time_out[i+1] = overlap[i+1] + transf_buf[i+1]*window_long_prev[i+1]
			time_out[i+2] = overlap[i+2] + transf_buf[i+2]*window_long_prev[i+2]
			time_out[i+3] = overlap[i+3] + transf_buf[i+3]*window_long_prev[i+3]
		}

		for i = 0; i < nlong; i += 4 {
			overlap[i] = transf_buf[nlong+i] * window_long[nlong-1-i]
			overlap[i+1] = transf_buf[nlong+i+1] * window_long[nlong-2-i]
			overlap[i+2] = transf_buf[nlong+i+2] * window_long[nlong-3-i]
			overlap[i+3] = transf_buf[nlong+i+3] * window_long[nlong-4-i]
		}
	case LONG_START_SEQUENCE:
		imdct_long(fb, freq_in, transf_buf[:])

		for i = 0; i < nlong; i += 4 {
			time_out[i] = overlap[i] + transf_buf[i]*window_long_prev[i]
			time_out[i+1] = overlap[i+1] + transf_buf[i+1]*window_long_prev[i+1]
			time_out[i+2] = overlap[i+2] + transf_buf[i+2]*window_long_prev[i+2]
			time_out[i+3] = overlap[i+3] + transf_buf[i+3]*window_long_prev[i+3]
		}

		for i = 0; i < nflat_ls; i++ {
			overlap[i] = transf_buf[nlong+i]
		}
		for i = 0; i < nshort; i++ {
			overlap[nflat_ls+i] = transf_buf[nlong+nflat_ls+i] * window_short[nshort-i-1]
		}
		for i = 0; i < nflat_ls; i++ {
			overlap[nflat_ls+nshort+i] = 0
		}
	case EIGHT_SHORT_SEQUENCE:
		imdct(fb.mdct256, freq_in[0*nshort:], transf_buf[2*nshort*0:])
		imdct(fb.mdct256, freq_in[1*nshort:], transf_buf[2*nshort*1:])
		imdct(fb.mdct256, freq_in[2*nshort:], transf_buf[2*nshort*2:])
		imdct(fb.mdct256, freq_in[3*nshort:], transf_buf[2*nshort*3:])
		imdct(fb.mdct256, freq_in[4*nshort:], transf_buf[2*nshort*4:])
		imdct(fb.mdct256, freq_in[5*nshort:], transf_buf[2*nshort*5:])
		imdct(fb.mdct256, freq_in[6*nshort:], transf_buf[2*nshort*6:])
		imdct(fb.mdct256, freq_in[7*nshort:], transf_buf[2*nshort*7:])

		for i = 0; i < nflat_ls; i++ {
			time_out[i] = overlap[i]
		}
		for i = 0; i < nshort; i++ {
			time_out[nflat_ls+i] = overlap[nflat_ls+i] + (transf_buf[nshort*0+i] * window_short_prev[i])
			time_out[nflat_ls+1*nshort+i] = overlap[nflat_ls+nshort*1+i] + (transf_buf[nshort*1+i] * window_short[nshort-1-i]) + (transf_buf[nshort*2+i] * window_short[i])
			time_out[nflat_ls+2*nshort+i] = overlap[nflat_ls+nshort*2+i] + (transf_buf[nshort*3+i] * window_short[nshort-1-i]) + (transf_buf[nshort*4+i] * window_short[i])
			time_out[nflat_ls+3*nshort+i] = overlap[nflat_ls+nshort*3+i] + (transf_buf[nshort*5+i] * window_short[nshort-1-i]) + (transf_buf[nshort*6+i] * window_short[i])
			if i < trans {
				time_out[nflat_ls+4*nshort+i] = overlap[nflat_ls+nshort*4+i] + (transf_buf[nshort*7+i] * window_short[nshort-1-i]) + (transf_buf[nshort*8+i] * window_short[i])
			}
		}

		for i = 0; i < nshort; i++ {
			if i >= trans {
				overlap[nflat_ls+4*nshort+i-nlong] = (transf_buf[nshort*7+i] * window_short[nshort-1-i]) + (transf_buf[nshort*8+i] * window_short[i])
			}
			overlap[nflat_ls+5*nshort+i-nlong] = (transf_buf[nshort*9+i] * window_short[nshort-1-i]) + (transf_buf[nshort*10+i] * window_short[i])
			overlap[nflat_ls+6*nshort+i-nlong] = (transf_buf[nshort*11+i] * window_short[nshort-1-i]) + (transf_buf[nshort*12+i] * window_short[i])
			overlap[nflat_ls+7*nshort+i-nlong] = (transf_buf[nshort*13+i] * window_short[nshort-1-i]) + (transf_buf[nshort*14+i] * window_short[i])
			overlap[nflat_ls+8*nshort+i-nlong] = (transf_buf[nshort*15+i] * window_short[nshort-1-i])
		}
		for i = 0; i < nflat_ls; i++ {
			overlap[nflat_ls+nshort+i] = 0
		}
	case LONG_STOP_SEQUENCE:
		imdct_long(fb, freq_in, transf_buf[:])

		for i = 0; i < nflat_ls; i++ {
			time_out[i] = overlap[i]
		}
		for i = 0; i < nshort; i++ {
			time_out[nflat_ls+i] = overlap[nflat_ls+i] + (transf_buf[nflat_ls+i] * window_short_prev[i])
		}
		for i = 0; i < nflat_ls; i++ {
			time_out[nflat_ls+nshort+i] = overlap[nflat_ls+nshort+i] + transf_buf[nflat_ls+nshort+i]
		}
		for i = 0; i < nlong; i++ {
			overlap[i] = transf_buf[nlong+i] * window_long[nlong-1-i]
		}
	}

	return
}

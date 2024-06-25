package aac

import "fmt"

//Code from FAAD2 is copyright (c) Nero AG, www.nero.com

func to_PCM_double(d *Decoder, input [][]float64, channels, frame_length uint) []float64 {
	var sample_buffer = make([]float64, 2048)
	/*switch CONV(channels, d.downMatrix) {
	case CONV(1,0), CONV(1,1):
		for i := uint(0); i < frame_length; i++ {
			inp := input[0][i]
			sample_buffer[i][0] = inp*FLOAT_SCALE
		}
	}*/

	//fmt.Println(CONV(1, 1))
	switch channels {
	case 1:
	case 2:
		for i := uint(0); i < frame_length; i++ {
			inp0 := input[0][i]
			inp1 := input[1][i]
			sample_buffer[(i*2)+0] = inp0 * 1
			sample_buffer[(i*2)+1] = inp1 * 1
		}
	}

	fmt.Println(sample_buffer)

	return sample_buffer
}

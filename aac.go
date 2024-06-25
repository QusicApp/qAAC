package aac

// library to decode aac frames based on faad2

import (
	"bytes"
	"io"

	"github.com/Eyevinn/mp4ff/bits"
)

type stringerror string

func (err stringerror) Error() string {
	return string(err)
}

const ErrInvalidAudioConfig stringerror = "invalid audio config"

func decode_fil(rd Reader) {
	count, _ := rd.Read(4)
	if count == 15 {
		c, _ := rd.Read(8)
		count += c
	}
	extension_type, _ := rd.Read(4)
	switch extension_type {
	case FILL_DATA:
		d, _ := rd.Read(4)
		if d == 0 {
			rd.Read((int(count) - 2) * 8)
		}
	default:
	}
}

type Decoder struct {
	objectType,
	frequencyIndex,
	channelConfiguration,
	frameLengthFlag,
	dependsOnCoreCoder,
	extensionFlag uint

	reader                Reader
	prev_window_shape     [MAX_CHANNELS]uint
	time_out, fb_intermed [MAX_CHANNELS][]float64

	internal_channel [MAX_CHANNELS]uint

	fb *fb_info
}

type Reader struct {
	*bits.Reader
	sf_index, frame_length uint
}

func (r Reader) Skip(bits int) error {
	_, err := r.Read(bits)
	return err
}

func NewDecoder(config []byte) (*Decoder, error) {
	if len(config) != 2 {
		return nil, ErrInvalidAudioConfig
	}
	r := bits.NewReader(bytes.NewReader(config))
	d := &Decoder{
		objectType:           r.MustRead(5),
		frequencyIndex:       r.MustRead(4),
		channelConfiguration: r.MustRead(4),
		frameLengthFlag:      r.MustRead(1),
		dependsOnCoreCoder:   r.MustRead(1),
		extensionFlag:        r.MustRead(1),
	}
	d.reader = Reader{nil, d.frequencyIndex, 1024}
	if d.frameLengthFlag == 1 {
		d.reader.frame_length = 960
	}
	d.fb = filter_bank_init(d.reader.frame_length)
	return d, nil
}

func (d *Decoder) DecodeFrame(frame io.Reader) []float64 {
	d.reader.Reader = bits.NewReader(frame)
	elemType, _ := d.reader.Read(3)
	switch elemType {
	case CPE:
		/*coef1, coef2 := decode_cpe(rd)
		smp1, smp2 := make([]float64, 2048), make([]float64, 2048)

		imdct(new_mdct(rd.frame_length), coef1[:], smp1)
		imdct(new_mdct(rd.frame_length), coef2[:], smp2)*/
		decode_cpe(d)
	default:
		return nil
	}
	return to_PCM_double(d, d.time_out[:], d.channelConfiguration, d.reader.frame_length)
}

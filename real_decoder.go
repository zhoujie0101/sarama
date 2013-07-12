package kafka

import (
	"encoding/binary"
	"math"
)

type realDecoder struct {
	raw   []byte
	off   int
	stack []pushDecoder
}

func (rd *realDecoder) remaining() int {
	return len(rd.raw) - rd.off
}

// primitives

func (rd *realDecoder) getInt8() (int8, error) {
	if rd.remaining() < 1 {
		return -1, DecodingError{}
	}
	tmp := int8(rd.raw[rd.off])
	rd.off += 1
	return tmp, nil
}

func (rd *realDecoder) getInt16() (int16, error) {
	if rd.remaining() < 2 {
		return -1, DecodingError{}
	}
	tmp := int16(binary.BigEndian.Uint16(rd.raw[rd.off:]))
	rd.off += 2
	return tmp, nil
}

func (rd *realDecoder) getInt32() (int32, error) {
	if rd.remaining() < 4 {
		return -1, DecodingError{}
	}
	tmp := int32(binary.BigEndian.Uint32(rd.raw[rd.off:]))
	rd.off += 4
	return tmp, nil
}

func (rd *realDecoder) getInt64() (int64, error) {
	if rd.remaining() < 8 {
		return -1, DecodingError{}
	}
	tmp := int64(binary.BigEndian.Uint64(rd.raw[rd.off:]))
	rd.off += 8
	return tmp, nil
}

// arrays

func (rd *realDecoder) getInt32Array() ([]int32, error) {
	if rd.remaining() < 4 {
		return nil, DecodingError{}
	}
	n := int(binary.BigEndian.Uint32(rd.raw[rd.off:]))
	rd.off += 4

	var ret []int32 = nil
	if rd.remaining() < 4*n {
		return nil, DecodingError{}
	} else if n > 0 {
		ret = make([]int32, n)
		for i := range ret {
			ret[i] = int32(binary.BigEndian.Uint32(rd.raw[rd.off:]))
			rd.off += 4
		}
	}
	return ret, nil
}

func (rd *realDecoder) getArrayCount() (int, error) {
	if rd.remaining() < 4 {
		return -1, DecodingError{}
	}
	tmp := int(binary.BigEndian.Uint32(rd.raw[rd.off:]))
	rd.off += 4
	if tmp > rd.remaining() || tmp > 2*math.MaxUint16 {
		return -1, DecodingError{}
	}
	return tmp, nil
}

// misc

func (rd *realDecoder) getError() (KError, error) {
	val, err := rd.getInt16()
	return KError(val), err
}

func (rd *realDecoder) getString() (*string, error) {
	tmp, err := rd.getInt16()

	if err != nil {
		return nil, err
	}

	n := int(tmp)

	switch {
	case n < -1:
		return nil, DecodingError{}
	case n == -1:
		return nil, nil
	case n == 0:
		return new(string), nil
	case n > rd.remaining():
		return nil, DecodingError{}
	default:
		tmp := new(string)
		*tmp = string(rd.raw[rd.off : rd.off+n])
		rd.off += n
		return tmp, nil
	}
}

func (rd *realDecoder) getBytes() (*[]byte, error) {
	tmp, err := rd.getInt32()

	if err != nil {
		return nil, err
	}

	n := int(tmp)

	switch {
	case n < -1:
		return nil, DecodingError{}
	case n == -1:
		return nil, nil
	case n == 0:
		tmp := make([]byte, 0)
		return &tmp, nil
	case n > rd.remaining():
		return nil, DecodingError{}
	default:
		tmp := rd.raw[rd.off : rd.off+n]
		rd.off += n
		return &tmp, nil
	}
}

func (rd *realDecoder) getSubset(length int) (packetDecoder, error) {
	if rd.remaining() < length {
		return nil, DecodingError{"Not enough data for subset."}
	}

	return &realDecoder{raw: rd.raw[rd.off : rd.off+length]}, nil
}

// stackable

func (rd *realDecoder) push(in pushDecoder) error {
	in.saveOffset(rd.off)

	if rd.remaining() < in.reserveLength() {
		return DecodingError{}
	}

	rd.stack = append(rd.stack, in)

	return nil
}

func (rd *realDecoder) pushLength32() error {
	return rd.push(&length32Decoder{})
}

func (rd *realDecoder) pushCRC32() error {
	return rd.push(&crc32Decoder{})
}

func (rd *realDecoder) pop() error {
	// this is go's ugly pop pattern (the inverse of append)
	in := rd.stack[len(rd.stack)-1]
	rd.stack = rd.stack[:len(rd.stack)-1]

	return in.check(rd.off, rd.raw)
}
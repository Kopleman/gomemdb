package utils

import "errors"

const (
	decimalBase   = 10
	kibibyteShift = 10
	mebibyteShift = 20
	gibibyteShift = 30
)

func ParseSize(text string) (int, error) {
	if len(text) == 0 || text[0] < '0' || text[0] > '9' {
		return 0, errors.New("incorrect size")
	}

	idx := 0
	size := 0
	for idx < len(text) && text[idx] >= '0' && text[idx] <= '9' {
		number := int(text[idx] - '0')
		size = size*decimalBase + number
		idx++
	}

	parameter := text[idx:]
	switch parameter {
	case "GB", "Gb", "gb":
		return size << gibibyteShift, nil
	case "MB", "Mb", "mb":
		return size << mebibyteShift, nil
	case "KB", "Kb", "kb":
		return size << kibibyteShift, nil
	case "B", "b", "":
		return size, nil
	default:
		return 0, errors.New("incorrect size")
	}
}

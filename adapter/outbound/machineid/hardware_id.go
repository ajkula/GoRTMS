package machineid

import (
	"crypto/sha256"
	"encoding/hex"

	"github.com/ajkula/GoRTMS/domain/port/outbound"
	"github.com/denisbrodbeck/machineid"
)

type hardwareMachineID struct{}

func NewHardwareMachineID() outbound.MachineIDService {
	return &hardwareMachineID{}
}

func (h *hardwareMachineID) GetMachineID() (string, error) {
	rawID, err := machineid.ID()
	if err != nil {
		return "", err
	}

	hash := sha256.Sum256([]byte(rawID))
	return hex.EncodeToString(hash[:]), nil
}

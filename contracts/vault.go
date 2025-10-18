package contracts

import (
	"context"
	"fmt"
	"math/big"
	"strings"

	"github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/ethclient"
)

// VaultContract wraps the VaultATFi smart contract interactions
type VaultContract struct {
	client   *ethclient.Client
	address  common.Address
	abi      abi.ABI
}

// NewVaultContract creates a new VaultContract instance
func NewVaultContract(client *ethclient.Client, address string) (*VaultContract, error) {
	// VaultATFi ABI - only the functions we need
	vaultABI := `[{"inputs":[],"name":"getParticipantCount","outputs":[{"internalType":"uint256","name":"","type":"uint256"}],"stateMutability":"view","type":"function"}]`

	parsedABI, err := abi.JSON(strings.NewReader(vaultABI))
	if err != nil {
		return nil, fmt.Errorf("failed to parse vault ABI: %w", err)
	}

	return &VaultContract{
		client:  client,
		address: common.HexToAddress(address),
		abi:     parsedABI,
	}, nil
}

// GetParticipantCount calls the getParticipantCount() function on the vault contract
func (vc *VaultContract) GetParticipantCount(ctx context.Context) (*big.Int, error) {
	callData, err := vc.abi.Pack("getParticipantCount")
	if err != nil {
		return nil, fmt.Errorf("failed to pack call data: %w", err)
	}

	result, err := vc.client.CallContract(ctx, ethereum.CallMsg{
		To:   &vc.address,
		Data: callData,
	}, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to call getParticipantCount: %w", err)
	}

	var participantCount *big.Int
	err = vc.abi.UnpackIntoInterface(&participantCount, "getParticipantCount", result)
	if err != nil {
		return nil, fmt.Errorf("failed to unpack result: %w", err)
	}

	return participantCount, nil
}

// GetEventDetails calls multiple view functions to get event details
func (vc *VaultContract) GetEventDetails(ctx context.Context) (map[string]interface{}, error) {
	// This can be extended to call other view functions like eventId, organizer, etc.
	participantCount, err := vc.GetParticipantCount(ctx)
	if err != nil {
		return nil, err
	}

	return map[string]interface{}{
		"participant_count": participantCount,
	}, nil
}
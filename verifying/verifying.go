package verifying

import (
	"errors"
	"fmt"

	"go.uber.org/zap"

	"github.com/spacemeshos/post/config"
	"github.com/spacemeshos/post/internal/postrs"
	"github.com/spacemeshos/post/oracle"
	"github.com/spacemeshos/post/shared"
)

// VerifyVRFNonce ensures the validity of a nonce for a given node.
// AtxId is the id of the ATX that was selected by the node for its commitment.
func VerifyVRFNonce(nonce *uint64, m *shared.VRFNonceMetadata, opts ...OptionFunc) error {
	if nonce == nil {
		return errors.New("invalid `nonce` value; expected: non-nil, given: nil")
	}

	if len(m.NodeId) != 32 {
		return fmt.Errorf("invalid `nodeId` length; expected: 32, given: %v", len(m.NodeId))
	}

	if len(m.CommitmentAtxId) != 32 {
		return fmt.Errorf("invalid `commitmentAtxId` length; expected: 32, given: %v", len(m.CommitmentAtxId))
	}

	options := defaultOpts()
	for _, opt := range opts {
		if err := opt(options); err != nil {
			return err
		}
	}

	numLabels := uint64(m.NumUnits) * uint64(m.LabelsPerUnit)
	difficulty := shared.PowDifficulty(numLabels)

	wo, err := oracle.New(
		oracle.WithCommitment(oracle.CommitmentBytes(m.NodeId, m.CommitmentAtxId)),
		oracle.WithScryptParams(options.labelScrypt),
		oracle.WithVRFDifficulty(difficulty),
	)
	if err != nil {
		return err
	}
	defer wo.Close()

	res, err := wo.Position(*nonce)
	if err != nil {
		return err
	}

	if res.Nonce == nil || *res.Nonce != *nonce {
		return fmt.Errorf("nonce %v is not valid for node %v", *nonce, m.NodeId)
	}

	return nil
}

type ProofVerifier struct {
	inner *postrs.Verifier
}

// NewProofVerifier creates a new proof verifier.
// If powFlags is nil, the recommended PoW flags will be used.
// The verifier must be closed after use with Close().
func NewProofVerifier(powFlags *config.PowFlags) (*ProofVerifier, error) {
	var flags config.PowFlags
	if powFlags == nil {
		flags = config.DefaultVerifyingPowFlags()
	} else {
		flags = *powFlags
	}
	inner, err := postrs.NewVerifier(flags)
	if err != nil {
		return nil, err
	}

	return &ProofVerifier{inner: inner}, nil
}

func (v *ProofVerifier) Close() error {
	return v.inner.Close()
}

// Verify ensures the validity of a proof in respect to its metadata.
// It returns nil if the proof is valid or an error describing the failure, otherwise.
func (v *ProofVerifier) Verify(p *shared.Proof, m *shared.ProofMetadata, cfg config.Config, logger *zap.Logger, opts ...OptionFunc) error {
	options := defaultOpts()
	for _, opt := range opts {
		if err := opt(options); err != nil {
			return err
		}
	}
	if len(m.NodeId) != 32 {
		return fmt.Errorf("invalid `nodeId` length; expected: 32, given: %v", len(m.NodeId))
	}
	if len(m.CommitmentAtxId) != 32 {
		return fmt.Errorf("invalid `commitmentAtxId` length; expected: 32, given: %v", len(m.CommitmentAtxId))
	}

	scryptParams := postrs.TranslateScryptParams(options.labelScrypt.N, options.labelScrypt.R, options.labelScrypt.P)
	return v.inner.VerifyProof(p, m, logger, cfg.K1, cfg.K2, cfg.K3, cfg.PowDifficulty, scryptParams)
}

package main

import (
	"encoding/binary"
	"encoding/hex"
	"flag"
	"io"
	"log"
	"math"
	"net"
	"time"

	"github.com/davecgh/go-spew/spew"
	"github.com/spacemeshos/post/internal/postrs"
	"github.com/spacemeshos/post/shared"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

func main() {
	var providerID uint64
	var printProviders bool
	var benchProviders bool
	var targetServer string
	var preferredSize uint64

	zapCfg := zap.Config{
		Level:    zap.NewAtomicLevelAt(zap.InfoLevel),
		Encoding: "console",
		EncoderConfig: zapcore.EncoderConfig{
			TimeKey:        "T",
			LevelKey:       "L",
			NameKey:        "N",
			MessageKey:     "M",
			LineEnding:     zapcore.DefaultLineEnding,
			EncodeLevel:    zapcore.CapitalLevelEncoder,
			EncodeTime:     zapcore.ISO8601TimeEncoder,
			EncodeDuration: zapcore.StringDurationEncoder,
		},
		OutputPaths:      []string{"stdout"},
		ErrorOutputPaths: []string{"stderr"},
	}

	logger, err := zapCfg.Build()
	if err != nil {
		log.Fatalln("failed to initialize zap logger:", err)
	}

	flag.Uint64Var(&providerID, "provider", 0, "compute provider id (required)")
	flag.BoolVar(&printProviders, "printProviders", false, "print the list of compute providers")
	flag.BoolVar(&benchProviders, "benchProviders", false, "benchmark the compute providers")
	flag.StringVar(&targetServer, "targetServer", "", "target server for remote computation")
	flag.Uint64Var(&preferredSize, "preferredSize", 16, "preferred size for remote computation")
	flag.Parse()

	if printProviders {
		providers, err := postrs.OpenCLProviders()
		if err != nil {
			log.Fatalln("failed to get OpenCL providers", err)
		}
		spew.Dump(providers)
		return
	}

	for {
		conn, err := net.Dial("tcp", targetServer)
		if err != nil {
			logger.Info("failed to connect to target server", zap.Error(err), zap.String("target", targetServer))
			time.Sleep(1 * time.Second)
			continue
		}
		// first send the key
		_, err = conn.Write([]byte("KEY ScryptServer"))
		if err != nil {
			logger.Info("failed to send key", zap.Error(err))
			conn.Close()
			continue
		}
		wConn := wireConn{conn}
		// then put the preferred size
		err = wConn.WriteUint64(preferredSize)
		if err != nil {
			logger.Info("failed to send preferred size", zap.Error(err))
			conn.Close()
			continue
		}
		// then read commitment
		var commitment [32]byte
		_, err = io.ReadFull(conn, commitment[:])
		if err != nil {
			logger.Info("failed to read commitment", zap.Error(err))
			conn.Close()
			continue
		}
		numLabels, err := wConn.ReadUint64()
		if err != nil {
			logger.Info("failed to read numLabels", zap.Error(err))
			conn.Close()
			continue
		}
		logger.Info("ready for work",
			zap.String("commitment", hex.EncodeToString(commitment[:])),
			zap.Uint64("numLabels", numLabels),
		)
		scrypter, err := postrs.NewScrypt(
			postrs.WithProviderID(uint32(providerID)),
			postrs.WithCommitment(commitment[:]),
			postrs.WithScryptN(8192),
			postrs.WithVRFDifficulty(shared.PowDifficulty(numLabels)),
			postrs.WithLogger(logger),
		)
		if err != nil {
			log.Println(err)
			conn.Close()
			continue
		}

		// init scrypter
		for {
			start, err := wConn.ReadUint64()
			if err != nil {
				logger.Info("failed to read start", zap.Error(err))
				break
			}
			end, err := wConn.ReadUint64()
			if err != nil {
				logger.Info("failed to read end", zap.Error(err))
				break
			}
			logger.Info("task started", zap.Uint64("start", start), zap.Uint64("end", end))
			t := time.Now()
			result, err := scrypter.Positions(start, end)
			if err != nil {
				logger.Info("failed to compute positions", zap.Error(err))
				break
			}
			logger.Info("task completed", zap.Float64("duration", time.Since(t).Seconds()), zap.Int("output_size", len(result.Output)))
			idxSolution := uint64(math.MaxUint64)
			if result.IdxSolution != nil {
				idxSolution = *result.IdxSolution
			}
			err = wConn.WriteUint64(idxSolution)
			if err != nil {
				logger.Info("failed to write idxSolution", zap.Error(err))
				break
			}
			_, err = conn.Write(result.Output)
			if err != nil {
				logger.Info("failed to write output", zap.Error(err))
				break
			}
		}
		conn.Close()
		// deinit scrypter
	}

}

type wireConn struct {
	conn net.Conn
}

func (c wireConn) ReadUint64() (uint64, error) {
	var buf [8]byte
	_, err := io.ReadFull(c.conn, buf[:])
	if err != nil {
		return 0, err
	}
	return binary.LittleEndian.Uint64(buf[:]), nil
}

func (c wireConn) WriteUint64(v uint64) error {
	var buf [8]byte
	binary.LittleEndian.PutUint64(buf[:], v)
	_, err := c.conn.Write(buf[:])
	return err
}

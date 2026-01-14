package cmd

import (
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"strings"

	"github.com/nsevo/v2sp/common/crypt"
	"github.com/spf13/cobra"
	"golang.org/x/crypto/curve25519"
)

var (
	realityNodeID   int
	realityNodeType string
	realityToken    string
)

var realityCommand = cobra.Command{
	Use:   "reality",
	Short: "Generate Reality x25519 key pair (private/public)",
	Long:  "Generate x25519 key pair used by Reality. If --node-id/--node-type/--token are provided, private key is derived deterministically.",
	Run: func(_ *cobra.Command, _ []string) {
		privateKey := make([]byte, curve25519.ScalarSize)

		// Deterministic mode (aligns with existing x25519 interactive feature)
		if realityNodeID > 0 && realityNodeType != "" && realityToken != "" {
			seed := fmt.Sprintf("%d%s%s", realityNodeID, strings.ToLower(realityNodeType), realityToken)
			privateKey = crypt.GenX25519Private([]byte(seed))
		} else {
			if _, err := rand.Read(privateKey); err != nil {
				fmt.Println(Err("read rand error: ", err))
				return
			}
		}

		publicKey, err := curve25519.X25519(privateKey, curve25519.Basepoint)
		if err != nil {
			fmt.Println(Err("gen X25519 error: ", err))
			return
		}

		fmt.Printf("Private key: %s\n", base64.RawURLEncoding.EncodeToString(privateKey))
		fmt.Printf("Public key: %s\n", base64.RawURLEncoding.EncodeToString(publicKey))
	},
	Args: cobra.NoArgs,
}

func init() {
	realityCommand.Flags().IntVar(&realityNodeID, "node-id", 0, "node id (for deterministic key derivation)")
	realityCommand.Flags().StringVar(&realityNodeType, "node-type", "", "node type (for deterministic key derivation)")
	realityCommand.Flags().StringVar(&realityToken, "token", "", "api token (for deterministic key derivation)")
	command.AddCommand(&realityCommand)
}

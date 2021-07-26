// Copyright The Linux Foundation and each contributor to CommunityBridge.
// SPDX-License-Identifier: MIT

package gitlab

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"

	log "github.com/communitybridge/easycla/cla-backend-go/logging"

	"github.com/xanzy/go-gitlab"
)

// OauthSuccessResponse is success response from Gitlab
type OauthSuccessResponse struct {
	AccessToken  string `json:"access_token"`
	TokenType    string `json:"token_type"`
	ExpiresIn    int    `json:"expires_in"`
	RefreshToken string `json:"refresh_token"`
	CreatedAt    int    `json:"created_at"`
}

// NewGitlabOauthClient creates a new gitlab client from the given oauth info, authInfo is encrypted
func NewGitlabOauthClient(authInfo string) (*gitlab.Client, error) {
	oauthResp, err := DecryptAuthInfo(authInfo)
	if err != nil {
		return nil, err
	}

	log.Infof("creating oauth client with access token : %s", oauthResp.AccessToken)
	return gitlab.NewOAuthClient(oauthResp.AccessToken)
}

// EncryptAuthInfo encrypts the oauth response into a string
func EncryptAuthInfo(oauthResp *OauthSuccessResponse) (string, error) {
	key := getGitlabAppPrivateKey()
	keyDecoded, err := base64.StdEncoding.DecodeString(key)
	if err != nil {
		return "", fmt.Errorf("decode key : %v", err)
	}

	b, err := json.Marshal(oauthResp)
	if err != nil {
		return "", fmt.Errorf("oauth resp json marshall : %v", err)
	}
	authInfo := string(b)
	//log.Infof("auth info before encrypting : %s", authInfo)

	encrypted, err := encrypt(keyDecoded, []byte(authInfo))
	if err != nil {
		return "", fmt.Errorf("encrypt failed : %v", err)
	}

	return hex.EncodeToString(encrypted), nil
}

// DecryptAuthInfo decrytps the authinfo into OauthSuccessResponse data structure
func DecryptAuthInfo(authInfoEncoded string) (*OauthSuccessResponse, error) {
	ciphertext, err := hex.DecodeString(authInfoEncoded)
	if err != nil {
		return nil, fmt.Errorf("decode auth info %s : %v", authInfoEncoded, err)
	}

	//log.Infof("auth info decoded : %s", ciphertext)

	key := getGitlabAppPrivateKey()
	keyDecoded, err := base64.StdEncoding.DecodeString(key)
	if err != nil {
		return nil, fmt.Errorf("decode key : %v", err)
	}

	//log.Debugf("before decrypt : keyDecoded : %s, cipherText : %s", keyDecoded, ciphertext)
	decrypted, err := decrypt(keyDecoded, ciphertext)
	if err != nil {
		return nil, fmt.Errorf("decrypt failed : %v", err)
	}
	//log.Debugf("after decrypt : keyDecoded : %s, decrypted : %s", keyDecoded, decrypted)

	var oauthResp OauthSuccessResponse
	if err := json.Unmarshal(decrypted, &oauthResp); err != nil {
		return nil, fmt.Errorf("unmarshall auth info : %v", err)
	}

	return &oauthResp, nil
}

func encrypt(key, message []byte) ([]byte, error) {
	// Initialize block cipher
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}

	// Create the byte slice that will hold encrypted message
	cipherText := make([]byte, aes.BlockSize+len(message))

	// Generate the Initialization Vector (IV) nonce
	// which is stored at the beginning of the byte slice
	// The IV is the same length as the AES blocksize
	iv := cipherText[:aes.BlockSize]
	_, err = io.ReadFull(rand.Reader, iv)
	if err != nil {
		return nil, err
	}

	// Choose the block cipher mode of operation
	// Using the cipher feedback (CFB) mode here.
	// CBCEncrypter also available.
	cfb := cipher.NewCFBEncrypter(block, iv)
	// Generate the encrypted message and store it
	// in the remaining bytes after the IV nonce
	cfb.XORKeyStream(cipherText[aes.BlockSize:], message)

	return cipherText, nil
}

// AES decryption
func decrypt(key, cipherText []byte) ([]byte, error) {
	// Initialize block cipher
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}

	// Separate the IV nonce from the encrypted message bytes
	iv := cipherText[:aes.BlockSize]
	cipherText = cipherText[aes.BlockSize:]

	// Decrypt the message using the CFB block mode
	cfb := cipher.NewCFBDecrypter(block, iv)
	cfb.XORKeyStream(cipherText, cipherText)

	return cipherText, nil
}

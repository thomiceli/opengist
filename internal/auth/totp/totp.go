package totp

import (
	"bytes"
	"crypto/rand"
	"encoding/base64"
	"html/template"
	"image/png"
	"strings"

	"github.com/pquerna/otp/totp"
)

const secretSize = 16

func GenerateQRCode(username, siteUrl string, secret []byte) (string, template.URL, []byte, error) {
	var err error
	if secret == nil {
		secret, err = generateSecret()
		if err != nil {
			return "", "", nil, err
		}
	}

	otpKey, err := totp.Generate(totp.GenerateOpts{
		SecretSize:  secretSize,
		Issuer:      "Opengist (" + strings.ReplaceAll(siteUrl, ":", "") + ")",
		AccountName: username,
		Secret:      secret,
	})
	if err != nil {
		return "", "", nil, err
	}

	qrcode, err := otpKey.Image(320, 240)
	if err != nil {
		return "", "", nil, err
	}

	var imgBytes bytes.Buffer
	if err = png.Encode(&imgBytes, qrcode); err != nil {
		return "", "", nil, err
	}

	qrcodeImage := template.URL("data:image/png;base64," + base64.StdEncoding.EncodeToString(imgBytes.Bytes()))

	return otpKey.Secret(), qrcodeImage, secret, nil
}

func Validate(passcode, secret string) bool {
	return totp.Validate(passcode, secret)
}

func generateSecret() ([]byte, error) {
	secret := make([]byte, secretSize)
	_, err := rand.Reader.Read(secret)
	if err != nil {
		return nil, err
	}

	return secret, nil
}

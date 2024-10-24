package totp

import (
	"bytes"
	"crypto/rand"
	"encoding/base64"
	"github.com/pquerna/otp/totp"
	"html/template"
	"image/png"
	"strings"
)

const secretSize = 16

func GenerateQRCode(username, siteUrl string, secret []byte) (string, template.URL, error, []byte) {
	var err error
	if secret == nil {
		secret, err = generateSecret()
		if err != nil {
			return "", "", err, nil
		}
	}

	otpKey, err := totp.Generate(totp.GenerateOpts{
		SecretSize:  secretSize,
		Issuer:      "Opengist (" + strings.ReplaceAll(siteUrl, ":", "") + ")",
		AccountName: username,
		Secret:      secret,
	})
	if err != nil {
		return "", "", err, nil
	}

	qrcode, err := otpKey.Image(320, 240)
	if err != nil {
		return "", "", err, nil
	}

	var imgBytes bytes.Buffer
	if err = png.Encode(&imgBytes, qrcode); err != nil {
		return "", "", err, nil
	}

	qrcodeImage := template.URL("data:image/png;base64," + base64.StdEncoding.EncodeToString(imgBytes.Bytes()))

	return otpKey.Secret(), qrcodeImage, nil, secret
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

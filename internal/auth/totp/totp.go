package totp

import (
	"bytes"
	"encoding/base64"
	"github.com/pquerna/otp/totp"
	"html/template"
	"image/png"
	"strings"
)

func GenerateQRCode(username, siteUrl string) (string, template.URL, error) {
	otpKey, err := totp.Generate(totp.GenerateOpts{
		SecretSize:  16,
		Issuer:      "Opengist (" + strings.ReplaceAll(siteUrl, ":", "") + ")",
		AccountName: username,
	})
	if err != nil {
		return "", "", err
	}

	qrcode, err := otpKey.Image(320, 240)
	if err != nil {
		return "", "", err
	}

	var imgBytes bytes.Buffer
	if err = png.Encode(&imgBytes, qrcode); err != nil {
		return "", "", err
	}

	qrcodeImage := template.URL("data:image/png;base64," + base64.StdEncoding.EncodeToString(imgBytes.Bytes()))

	return otpKey.Secret(), qrcodeImage, nil
}

func Validate(passcode, secret string) bool {
	return totp.Validate(passcode, secret)
}

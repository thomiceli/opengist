# Custom assets

To add custom assets to your Opengist instance, you can use the `$opengist-home/custom` directory (where `$opengist-home` is the directory where Opengist stores its data).

### Logo / Favicon

To add a custom logo or favicon, you can add your own image file to the `$opengist-home/custom` directory, then define the relative path in the config.

For example, if you have a logo file `logo.png` in the `$opengist-home/custom` directory, you can set the logo path in the config as follows:

#### YAML
```yaml
custom.logo: logo.png
```

#### Environment variable
```sh
export OG_CUSTOM_LOGO=logo.png
```

Same as the favicon:

#### YAML
```yaml
custom.favicon: favicon.png
```

#### Environment variable
```sh
export OG_CUSTOM_FAVICON=favicon.png
```
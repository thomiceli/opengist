# Custom links

On the footer of your Opengist instance, you can add links to custom static templates or any other website you want to link to.
This can be useful for legal information, privacy policy, or any other information you want to provide to your users.

To add one or more links, you can add your own file to the `$opengist-home/custom` directory or set a URL, then define the relative path and its name in the config.

For example, if you have a legal information file `legal.html` in the `$opengist-home/custom` directory, and also wish to add a link to a Gitea instance, you can set the link in the config as follows:

#### YAML
```yaml
custom.static-links:
  - name: Legal notices
    path: legal.html
  - name: Gitea
    path: https://gitea.com
```

#### Environment variable
```sh
OG_CUSTOM_STATIC_LINK_0_NAME="Legal Notices" \
OG_CUSTOM_STATIC_LINK_0_PATH=legal.html \
OG_CUSTOM_STATIC_LINK_1_NAME=Gitea \
OG_CUSTOM_STATIC_LINK_1_PATH=https://gitea.com \
./opengist
```

## Templating custom HTML pages

In the start and end of the custom HTML files, you can use the syntax to include the header and footer of the Opengist instance:

```html
{{ template "header" . }}

<!-- my content -->

{{ template "footer" . }}
```
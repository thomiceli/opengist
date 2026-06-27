# Access tokens

Access tokens are used to access your private gists and their raw content. For now, it is the only use while a future API is being developed.

## Creating an access token

To create an access token, follow these steps:
1. Go to Settings
2. Select the "Access Tokens" menu
3. Choose a name for your token, the scope and an expiration date (optional), then click "Create Access Token"

## Using an access token

Once you have created an access token, you can use it to access your private gists with it.

Replace `<token>` with your actual access token in the following examples.

```shell
# Access raw content of a private gist, latest revision for "file.txt". Note: this URL is obtained from the "Raw" button on the gist page.
curl -H "Authorization: Token <token>" \
 http://opengist.example.com/user/gist/raw/HEAD/file.txt

# Access the JSON representation of a private gist. See "Gist as JSON" documentation for more details.
curl -H "Authorization: Token <token>" \
 http://opengist.example.com/user/gist.json
```

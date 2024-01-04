# Retrieve Gist as JSON

To retrieve a Gist as JSON, you can add `.json` to the end of the URL of your gist:

```shell
curl http://opengist.url/thomas/my-gist.json | jq '.'
```

It returns a JSON object with the following structure similar to this one:
```json
{
  "created_at": "2023-04-12T13:15:20+02:00",
  "description": "",
  "embed": {
    "css": "http://localhost:6157/assets/embed-94abc261.css",
    "html": "<div class=\"opengist-embed\" id=\"my-gist\">\n    <div class=\"html \">\n    \n        <div class=\"rounded-md border-1 border-gray-100 dark:border-gray-800 overflow-auto mb-4\">\n            <div class=\"border-b-1 border-gray-100 dark:border-gray-700 text-xs p-2 pl-4 bg-gray-50 dark:bg-gray-800 text-gray-400\">\n                <a target=\"_blank\" href=\"http://localhost:6157/thomas/my-gist#file-hello-md\"><span class=\"font-bold text-gray-700 dark:text-gray-200\">hello.md</span> · 21 B · Markdown</a>\n                <span class=\"float-right\"><a target=\"_blank\" href=\"http://localhost:6157\">Hosted via Opengist</a> · <span class=\"text-gray-700 dark:text-gray-200 font-bold\"><a target=\"_blank\" href=\"http://localhost:6157/thomas/my-gist/raw/HEAD/hello.md\">view raw</a></span></span>\n            </div>\n            \n            \n            \n            <div class=\"chroma markdown markdown-body p-8\"><h1>Welcome to Opengist</h1>\n</div>\n            \n\n        </div>\n    \n    </div>\n</div>\n",
    "js": "http://localhost:6157/thomas/my-gist.js",
    "js_dark": "http://localhost:6157/thomas/my-gist.js?dark"
  },
  "files": [
    {
      "filename": "hello.md",
      "size": 21,
      "human_size": "21 B",
      "content": "# Welcome to Opengist",
      "truncated": false,
      "type": "Markdown"
    }
  ],
  "id": "my-gist",
  "owner": "thomas",
  "title": "hello.md",
  "uuid": "8622b297bce54b408e36d546cef8019d",
  "visibility": "public"
}
```


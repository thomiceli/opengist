# Embed a Gist to your webpage

> [!Tip]
> Fancy to enforce light or dark mode on the embedded Gist?
> Just append `?light` or `?dark` to the Gist-URL.
> Omitting this parameter will cause OpenGist to fallback to `auto`, thus the Browser deciding on the users preference.

To embed a Gist to your webpage, you can add a script tag with the URL of your gist followed by `.js` to your HTML page:
    
```html
<script src="http://opengist.url/user/gist-url.js"></script>

<!-- Dark mode: -->
<script src="http://opengist.url/user/gist-url.js?dark"></script>
<!-- Light mode: -->
<script src="http://opengist.url/user/gist-url.js?light"></script>
```

If you have a Gist that holds several different files, you can also explicitely call a specific file by its filename:

```html
<script src="http://opengist.url/user/gist-url.js?file=filename"></script>

<!-- Dark mode: -->
<script src="http://opengist.url/user/gist-url.js?file=filename&dark"></script>
<!-- Light mode: -->
<script src="http://opengist.url/user/gist-url.js?file=filename&light"></script>
```

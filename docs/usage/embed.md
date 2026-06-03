# Embed a Gist to your webpage

To embed a Gist to your webpage, you can add a script tag with the URL of your gist followed by `.js` to your HTML page:
    
```html
<script src="http://opengist.url/user/gist-url.js"></script>

<!-- Dark mode: -->
<script src="http://opengist.url/user/gist-url.js?dark"></script>
```

If you have a Gist that holds several different files, you can also explicitely call a specific file by its filename:

```html
<script src="http://opengist.url/user/gist-url/filename.js"></script>

<!-- Dark mode: -->
<script src="http://opengist.url/user/gist-url/filename.js?dark"></script>
```

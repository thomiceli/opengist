import './style.css'
import './markdown.css'
import 'highlight.js/styles/tokyo-night-dark.css'
import moment from 'moment'
import md from 'markdown-it'
import hljs from 'highlight.js'

document.addEventListener('DOMContentLoaded', () => {
    document.querySelectorAll('.moment-timestamp').forEach((e) => {
        e.title = moment.unix(e.innerHTML).format('LLLL')
        e.innerHTML = moment.unix(e.innerHTML).fromNow()
    })

    document.querySelectorAll('.moment-timestamp-date').forEach((e) => {
        e.innerHTML = moment.unix(e.innerHTML).format('DD/MM/YYYY HH:mm')
    })

    let rev = document.querySelector('.revision-text')
    if (rev) {
        let fullRev = rev.innerHTML
        let smallRev = fullRev.substring(0, 8)
        rev.innerHTML = smallRev

        rev.onmouseover = () => {
            rev.innerHTML = fullRev
        }
        rev.onmouseout = () => {
            rev.innerHTML = smallRev
        }
    }

    document.querySelectorAll('.markdown').forEach((e) => {
        e.innerHTML = md().render(e.innerHTML);
    })

    document.querySelectorAll('.table-code').forEach((el) => {
        let ext = el.dataset.filename.split('.').pop()
        if (ext!== 'txt') {
        
            el.querySelectorAll('td.line-code').forEach((ell) => {
                ell.classList.add('language-'+ext)
                hljs.highlightElement(ell);
            });
        }

        // more efficient
        el.addEventListener('click', event => {
            if (event.target.matches('.line-num')) {
                Array.from(document.querySelectorAll('.table-code .selected')).forEach((el) => el.classList.remove('selected'));

                event.target.nextSibling.classList.add('selected')

                let filename = el.dataset.filenameSlug
                let line = event.target.textContent
                let url = location.protocol + '//' + location.host + location.pathname
                let hash = '#file-'+ filename + '-' +line
                window.history.pushState(null, null, url+hash);
                location.hash = hash;
            }
        });
    });


    let colorhash = () => {
        Array.from(document.querySelectorAll('.table-code .selected')).forEach((el) => el.classList.remove('selected'));
        let lineEl = document.querySelector(location.hash)
        if (lineEl) {
            lineEl.nextSibling.classList.add('selected')
        }
    }

    if (location.hash) {
        colorhash()
    }
    window.onhashchange = colorhash

    document.getElementById('main-menu-button').onclick = () => {
        document.getElementById('mobile-menu').classList.toggle('hidden')
    }

    let tabs = document.getElementById('gist-tabs')
    if (tabs) {
        tabs.onchange = (e) => {
            // navigate to the url in data-url
            window.location.href = e.target.selectedOptions[0].dataset.url
        }
    }

    let gistmenutoggle = document.getElementById('gist-menu-toggle');
    if (gistmenutoggle) {
        let gistmenucopy = document.getElementById('gist-menu-copy')
        let gistmenubuttoncopy = document.getElementById('gist-menu-button-copy')
        let gistmenuinput = document.getElementById('gist-menu-input')
        let gistmenutitle = document.getElementById('gist-menu-title')
        gistmenutitle.textContent = gistmenucopy.children[0].firstChild.textContent
        gistmenuinput.value = gistmenucopy.children[0].dataset.link

        gistmenutoggle.onclick = () => {
            gistmenucopy.classList.toggle('hidden')
        }

        for (let item of gistmenucopy.children) {
            item.onclick = () => {
                gistmenutitle.textContent = item.firstChild.textContent
                gistmenuinput.value = item.dataset.link
                gistmenucopy.classList.toggle('hidden')
            }
        }

        gistmenubuttoncopy.onclick = () => {
            let text = gistmenuinput.value
            navigator.clipboard.writeText(text).then(null, function(err) {
                console.error('Could not copy text: ', err);
            })
        }
    }

    let sortgist = document.getElementById('sort-gists-button')
    if (sortgist) {
        sortgist.onclick = () => {
            document.getElementById('sort-gists-dropdown').classList.toggle('hidden')
        }
    }

    document.querySelectorAll('.copy-gist-btn').forEach((e) => {
        e.onclick = () => {
            navigator.clipboard.writeText(e.parentNode.querySelector('.gist-content').textContent).then(null, function (err) {
                console.error('Could not copy text: ', err);
            })
        }
    })



});
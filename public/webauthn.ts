let loginMethod = "login"

function encodeArrayBufferToBase64Url(buffer) {
    const base64 = btoa(String.fromCharCode.apply(null, new Uint8Array(buffer)));

    return base64.replace(/\+/g, '-').replace(/\//g, '_').replace(/=+$/, '');
}

function decodeBase64UrlToArrayBuffer(base64Url) {
    let base64 = base64Url.replace(/-/g, '+').replace(/_/g, '/');
    while (base64.length % 4) {
        base64 += '=';
    }
    const binaryString = atob(base64);
    const buffer = new ArrayBuffer(binaryString.length);
    const view = new Uint8Array(buffer);
    for (let i = 0; i < binaryString.length; i++) {
        view[i] = binaryString.charCodeAt(i);
    }

    return buffer;
}

async function bindPasskey() {
    let waitText = document.getElementById("login-passkey-wait");

    try {
        this.classList.add('hidden');
        waitText.classList.remove('hidden');

        let csrf = document.querySelector<HTMLInputElement>('form#webauthn input[name="_csrf"]').value

        const beginResponse = await fetch('/webauthn/bind', {
            method: 'POST',
            credentials: 'include',
            body: new FormData(document.querySelector<HTMLFormElement>('form#webauthn'))
        });
        const beginData = await beginResponse.json();

        beginData.publicKey.challenge = decodeBase64UrlToArrayBuffer(beginData.publicKey.challenge);
        beginData.publicKey.user.id = decodeBase64UrlToArrayBuffer(beginData.publicKey.user.id);
        for (const cred of beginData.publicKey.excludeCredentials ?? []) {
            cred.id = decodeBase64UrlToArrayBuffer(cred.id);
        }


        const credential = await navigator.credentials.create({
            publicKey: beginData.publicKey,
        });

        if (!credential || !credential.rawId || !credential.response) {
            throw new Error('Credential object is missing required properties');
        }

        const finishResponse = await fetch('/webauthn/bind/finish', {
            method: 'POST',
            credentials: 'include',
            headers: {
                'Content-Type': 'application/json',
                'X-CSRF-Token': csrf
            },
            body: JSON.stringify({
                id: credential.id,
                rawId: encodeArrayBufferToBase64Url(credential.rawId),
                response: {
                    attestationObject: encodeArrayBufferToBase64Url(credential.response.attestationObject),
                    clientDataJSON: encodeArrayBufferToBase64Url(credential.response.clientDataJSON),
                },
                type: credential.type,
                passkeyname: document.querySelector<HTMLInputElement>('form#webauthn input[name="passkeyname"]').value
            }),
        });
        const finishData = await finishResponse.json();

        setTimeout(() => {
            window.location.reload();
        }, 100);
    } catch (error) {
        console.error('Error during passkey registration:', error);
        waitText.classList.add('hidden');
        this.classList.remove('hidden');
        alert(error);
    }
}

async function loginWithPasskey() {
    let waitText = document.getElementById("login-passkey-wait");

    try {
        this.classList.add('hidden');
        waitText.classList.remove('hidden');

        let csrf = document.querySelector<HTMLInputElement>('form#webauthn input[name="_csrf"]').value
        const beginResponse = await fetch('/webauthn/' + loginMethod, {
            method: 'POST',
            credentials: 'include',
            body: new FormData(document.querySelector<HTMLFormElement>('form#webauthn'))
        });
        const beginData = await beginResponse.json();

        beginData.publicKey.challenge = decodeBase64UrlToArrayBuffer(beginData.publicKey.challenge);

        if (beginData.publicKey.allowCredentials) {
            beginData.publicKey.allowCredentials = beginData.publicKey.allowCredentials.map(cred => ({
                ...cred,
                id: decodeBase64UrlToArrayBuffer(cred.id),
            }));
        }

        const credential = await navigator.credentials.get({
            publicKey: beginData.publicKey,
        });

        if (!credential || !credential.rawId || !credential.response) {
            throw new Error('Credential object is missing required properties');
        }

        const finishResponse = await fetch('/webauthn/' + loginMethod + '/finish', {
            method: 'POST',
            credentials: 'include',
            headers: {
                'Content-Type': 'application/json',
                'X-CSRF-Token': csrf
            },

            body: JSON.stringify({
                id: credential.id,
                rawId: encodeArrayBufferToBase64Url(credential.rawId),
                response: {
                    authenticatorData: encodeArrayBufferToBase64Url(credential.response.authenticatorData),
                    clientDataJSON: encodeArrayBufferToBase64Url(credential.response.clientDataJSON),
                    signature: encodeArrayBufferToBase64Url(credential.response.signature),
                    userHandle: encodeArrayBufferToBase64Url(credential.response.userHandle),
                },
                type: credential.type,
                clientExtensionResults: credential.getClientExtensionResults(),
            }),
        });
        const finishData = await finishResponse.json();

        if (!finishResponse.ok) {
            throw new Error(finishData.message || 'Unknown error');
        }

        setTimeout(() => {
            window.location.href = '/';
        }, 100);
    } catch (error) {
        console.error('Login error:', error);
        waitText.classList.add('hidden');
        this.classList.remove('hidden');
        alert(error);
    }
}

document.addEventListener('DOMContentLoaded', () => {
    const registerButton = document.getElementById('bind-passkey-button');
    if (registerButton) {
        registerButton.addEventListener('click', bindPasskey);
    }

    if (document.documentURI.includes('/mfa')) {
        loginMethod = "assertion"
    }

    const loginButton = document.getElementById('login-passkey-button');
    if (loginButton) {
        loginButton.addEventListener('click', loginWithPasskey);
    }
});

document.addEventListener('DOMContentLoaded', () => {
    let elems = Array.from(document.getElementsByClassName("toggle-button"));
    for (let elem of elems) {
        elem.addEventListener('click', () => {
            registerDomSetting(elem as HTMLElement)
        })
    }

    let copyInviteButtons = Array.from(document.getElementsByClassName("copy-invitation-link"));
    for (let button of copyInviteButtons) {
        button.addEventListener('click', () => {
            navigator.clipboard.writeText((button as HTMLElement).dataset.link).catch((err) => {
                console.error('Could not copy text: ', err);
            });
        })
    }

    // AI settings save button
    const saveAiSettingsBtn = document.getElementById('save-ai-settings');
    if (saveAiSettingsBtn) {
        saveAiSettingsBtn.addEventListener('click', saveAiSettings);
    }

    // AI API type change handler - show/hide API key field
    const aiApiTypeSelect = document.getElementById('ai-api-type');
    const aiApiKeyContainer = document.getElementById('ai-api-key-container');
    if (aiApiTypeSelect && aiApiKeyContainer) {
        aiApiTypeSelect.addEventListener('change', () => {
            const selectedType = (aiApiTypeSelect as HTMLSelectElement).value;
            if (selectedType === 'openai') {
                aiApiKeyContainer.classList.remove('hidden');
            } else {
                aiApiKeyContainer.classList.add('hidden');
            }
        });
    }
});

const setSetting = (key: string, value: string) => {
    // @ts-ignore
    const baseUrl = window.opengist_base_url || '';
    const data = new URLSearchParams();
    data.append('key', key);
    data.append('value', value);
    if (document.getElementsByName('_csrf').length !== 0) {
        data.append('_csrf', ((document.getElementsByName('_csrf')[0] as HTMLInputElement).value));
    }
    return fetch(`${baseUrl}/admin-panel/set-config`, {
        method: 'PUT',
        credentials: 'same-origin',
        body: data,
    });
};

const registerDomSetting = (el: HTMLElement) => {
    // @ts-ignore
    el.dataset["bool"] = !(el.dataset["bool"] === 'true');
    setSetting(el.id, el.dataset["bool"] === 'true' ? '1' : '0')
        .then(() => {
            el.classList.toggle("bg-primary-600");
            el.classList.toggle("dark:bg-gray-400");
            el.classList.toggle("bg-gray-300");
            (el.childNodes.item(1) as HTMLElement).classList.toggle("translate-x-5");
        });
};

const saveAiSettings = async () => {
    // @ts-ignore
    const baseUrl = window.opengist_base_url || '';
    const csrfToken = document.querySelector<HTMLInputElement>('input[name="_csrf"]')?.value || '';
    
    const aiEnabled = (document.getElementById('ai-enabled') as HTMLElement)?.dataset['bool'] === 'true' ? '1' : '0';
    const aiAPIType = (document.getElementById('ai-api-type') as HTMLSelectElement)?.value || 'ollama';
    const aiBaseURL = (document.getElementById('ai-base-url') as HTMLInputElement)?.value || '';
    const aiAPIKey = (document.getElementById('ai-api-key') as HTMLInputElement)?.value || '';
    const aiModel = (document.getElementById('ai-model') as HTMLInputElement)?.value || '';
    const aiSystemPrompt = (document.getElementById('ai-system-prompt') as HTMLTextAreaElement)?.value || '';
    const aiUserPrompt = (document.getElementById('ai-user-prompt-template') as HTMLTextAreaElement)?.value || '';
    
    const statusEl = document.getElementById('ai-save-status');
    if (statusEl) {
        statusEl.textContent = 'Saving...';
    }
    
    const settings = [
        { key: 'ai-enabled', value: aiEnabled },
        { key: 'ai-api-type', value: aiAPIType },
        { key: 'ai-base-url', value: aiBaseURL },
        { key: 'ai-api-key', value: aiAPIKey },
        { key: 'ai-model', value: aiModel },
        { key: 'ai-system-prompt', value: aiSystemPrompt },
        { key: 'ai-user-prompt-template', value: aiUserPrompt },
    ];
    
    try {
        for (const setting of settings) {
            const data = new URLSearchParams();
            data.append('key', setting.key);
            data.append('value', setting.value);
            data.append('_csrf', csrfToken);
            
            const response = await fetch(`${baseUrl}/admin-panel/set-config`, {
                method: 'PUT',
                credentials: 'same-origin',
                body: data,
            });
            
            if (!response.ok) {
                throw new Error(`Failed to save ${setting.key}`);
            }
        }
        
        if (statusEl) {
            statusEl.textContent = 'Saved!';
            setTimeout(() => {
                if (statusEl) statusEl.textContent = '';
            }, 2000);
        }
    } catch (error) {
        console.error('Error saving AI settings:', error);
        if (statusEl) {
            statusEl.textContent = 'Error saving';
        }
    }
};

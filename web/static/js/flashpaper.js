/**
 * FlashPaper - Client-side encryption for zero-knowledge paste sharing
 *
 * All encryption/decryption happens in the browser using the Web Crypto API.
 * The server never sees plaintext content - only encrypted ciphertext.
 *
 * Encryption: AES-256-GCM (Galois Counter Mode)
 * Key Derivation: PBKDF2-SHA256 with 100,000 iterations
 * Key Format: 256-bit random key encoded as Base58 in URL fragment
 */

const FlashPaper = (function() {
    'use strict';

    // Constants matching PrivateBin's encryption parameters
    const ITERATIONS = 100000;
    const KEY_SIZE = 256;
    const TAG_SIZE = 128;
    const ALGORITHM = 'aes';
    const MODE = 'gcm';

    // Base58 alphabet (Bitcoin style - no 0, O, I, l)
    const BASE58_ALPHABET = '123456789ABCDEFGHJKLMNPQRSTUVWXYZabcdefghijkmnopqrstuvwxyz';

    // Current paste state
    let currentPaste = null;
    let deleteToken = null;

    /**
     * Initialize FlashPaper - detect if viewing paste or creating new
     */
    function init() {
        // Initialize theme from localStorage or system preference
        initTheme();

        const pasteId = getPasteIdFromUrl();

        if (pasteId) {
            showViewMode();
            loadPaste(pasteId);
        } else {
            showCreateMode();
        }

        // Set up event listeners
        setupEventListeners();
    }

    // =====================
    // Theme Functions
    // =====================

    /**
     * Initialize theme from localStorage or system preference
     */
    function initTheme() {
        const savedTheme = localStorage.getItem('flashpaper-theme');

        if (savedTheme) {
            setTheme(savedTheme);
        } else {
            // Default to light mode (don't check system preference)
            setTheme('light');
        }
    }

    /**
     * Set the theme and update UI
     */
    function setTheme(theme) {
        const html = document.documentElement;
        const icon = document.getElementById('theme-icon');
        const text = document.getElementById('theme-text');

        if (theme === 'dark') {
            html.setAttribute('data-theme', 'dark');
            if (icon) icon.innerHTML = '&#9728;'; // Sun icon
            if (text) text.textContent = 'Light';
        } else {
            html.removeAttribute('data-theme');
            if (icon) icon.innerHTML = '&#9790;'; // Moon icon
            if (text) text.textContent = 'Dark';
        }

        localStorage.setItem('flashpaper-theme', theme);
    }

    /**
     * Toggle between light and dark theme
     */
    function toggleTheme() {
        const currentTheme = document.documentElement.getAttribute('data-theme');
        const newTheme = currentTheme === 'dark' ? 'light' : 'dark';
        setTheme(newTheme);
    }

    /**
     * Set up all event listeners
     */
    function setupEventListeners() {
        // Theme toggle button
        document.getElementById('theme-toggle')?.addEventListener('click', toggleTheme);

        // Create paste button
        document.getElementById('create-paste')?.addEventListener('click', createPaste);

        // Decrypt button (for password-protected pastes)
        document.getElementById('decrypt-btn')?.addEventListener('click', decryptWithPassword);

        // View burn-after-reading paste
        document.getElementById('view-burn')?.addEventListener('click', viewBurnPaste);

        // Clone paste
        document.getElementById('clone-paste')?.addEventListener('click', clonePaste);

        // Copy URL (now a button instead of anchor)
        document.getElementById('paste-url')?.addEventListener('click', copyUrl);

        // Delete paste
        document.getElementById('delete-paste')?.addEventListener('click', deletePaste);

        // Add comment
        document.getElementById('add-comment')?.addEventListener('click', addComment);

        // Burn after reading checkbox disables discussion
        document.getElementById('burn-after-reading')?.addEventListener('change', function() {
            const discussionCheckbox = document.getElementById('open-discussion');
            if (this.checked) {
                discussionCheckbox.checked = false;
                discussionCheckbox.disabled = true;
            } else {
                discussionCheckbox.disabled = false;
            }
        });

        // Handle Enter key in password field
        document.getElementById('decrypt-password')?.addEventListener('keypress', function(e) {
            if (e.key === 'Enter') decryptWithPassword();
        });

        // Handle Ctrl+Enter to submit paste
        document.getElementById('paste-content')?.addEventListener('keydown', function(e) {
            if (e.ctrlKey && e.key === 'Enter') createPaste();
        });
    }

    /**
     * Show create mode UI
     */
    function showCreateMode() {
        document.getElementById('new-paste').classList.remove('hidden');
        document.getElementById('view-paste').classList.add('hidden');
    }

    /**
     * Show view mode UI
     */
    function showViewMode() {
        document.getElementById('new-paste').classList.add('hidden');
        document.getElementById('view-paste').classList.remove('hidden');
    }

    /**
     * Get paste ID from URL query string
     */
    function getPasteIdFromUrl() {
        const query = window.location.search;
        if (query && query.length > 1) {
            // Remove leading '?' and any additional parameters
            return query.substring(1).split('&')[0];
        }
        return null;
    }

    /**
     * Get encryption key from URL fragment
     */
    function getKeyFromUrl() {
        const hash = window.location.hash;
        if (hash && hash.length > 1) {
            let key = hash.substring(1);
            // Handle burn-after-reading prefix
            if (key.startsWith('-')) {
                key = key.substring(1);
            }
            return base58Decode(key);
        }
        return null;
    }

    /**
     * Check if this is a burn-after-reading paste from URL
     */
    function isBurnFromUrl() {
        const hash = window.location.hash;
        return hash && hash.startsWith('#-');
    }

    // =====================
    // Encryption Functions
    // =====================

    /**
     * Generate random bytes using Web Crypto API
     */
    function getRandomBytes(length) {
        const array = new Uint8Array(length);
        crypto.getRandomValues(array);
        return array;
    }

    /**
     * Convert ArrayBuffer to Base64 string
     */
    function arrayBufferToBase64(buffer) {
        const bytes = new Uint8Array(buffer);
        let binary = '';
        for (let i = 0; i < bytes.byteLength; i++) {
            binary += String.fromCharCode(bytes[i]);
        }
        return btoa(binary);
    }

    /**
     * Convert Base64 string to ArrayBuffer
     */
    function base64ToArrayBuffer(base64) {
        const binary = atob(base64);
        const bytes = new Uint8Array(binary.length);
        for (let i = 0; i < binary.length; i++) {
            bytes[i] = binary.charCodeAt(i);
        }
        return bytes.buffer;
    }

    /**
     * Convert Uint8Array to string
     */
    function uint8ArrayToString(array) {
        return new TextDecoder().decode(array);
    }

    /**
     * Convert string to Uint8Array
     */
    function stringToUint8Array(str) {
        return new TextEncoder().encode(str);
    }

    /**
     * Base58 encode
     */
    function base58Encode(bytes) {
        if (bytes.length === 0) return '';

        // Convert to array if Uint8Array
        const input = Array.from(bytes);

        // Count leading zeros
        let zeros = 0;
        while (zeros < input.length && input[zeros] === 0) {
            zeros++;
        }

        // Convert to base58
        const encoded = [];
        let num = BigInt('0x' + Array.from(input).map(b => b.toString(16).padStart(2, '0')).join(''));

        while (num > 0n) {
            const remainder = Number(num % 58n);
            num = num / 58n;
            encoded.unshift(BASE58_ALPHABET[remainder]);
        }

        // Add leading '1's for leading zeros
        for (let i = 0; i < zeros; i++) {
            encoded.unshift(BASE58_ALPHABET[0]);
        }

        return encoded.join('');
    }

    /**
     * Base58 decode
     */
    function base58Decode(str) {
        if (!str || str.length === 0) return new Uint8Array(0);

        // Count leading '1's
        let zeros = 0;
        while (zeros < str.length && str[zeros] === '1') {
            zeros++;
        }

        // Convert from base58
        let num = 0n;
        for (const char of str) {
            const index = BASE58_ALPHABET.indexOf(char);
            if (index === -1) {
                throw new Error('Invalid Base58 character: ' + char);
            }
            num = num * 58n + BigInt(index);
        }

        // Convert BigInt to bytes
        let hex = num.toString(16);
        if (hex.length % 2) hex = '0' + hex;

        const bytes = new Uint8Array(zeros + hex.length / 2);
        for (let i = 0; i < hex.length / 2; i++) {
            bytes[zeros + i] = parseInt(hex.substr(i * 2, 2), 16);
        }

        return bytes;
    }

    /**
     * Derive encryption key using PBKDF2
     */
    async function deriveKey(keyBytes, password, salt) {
        // Combine key with password if provided
        let keyMaterial = keyBytes;
        if (password && password.length > 0) {
            const passwordBytes = stringToUint8Array(password);
            const combined = new Uint8Array(keyBytes.length + passwordBytes.length);
            combined.set(keyBytes);
            combined.set(passwordBytes, keyBytes.length);
            keyMaterial = combined;
        }

        // Import key material for PBKDF2
        const baseKey = await crypto.subtle.importKey(
            'raw',
            keyMaterial,
            'PBKDF2',
            false,
            ['deriveKey']
        );

        // Derive AES key
        return crypto.subtle.deriveKey(
            {
                name: 'PBKDF2',
                salt: salt,
                iterations: ITERATIONS,
                hash: 'SHA-256'
            },
            baseKey,
            {
                name: 'AES-GCM',
                length: KEY_SIZE
            },
            false,
            ['encrypt', 'decrypt']
        );
    }

    /**
     * Encrypt data using AES-256-GCM
     */
    async function encrypt(plaintext, password) {
        // Generate random key (32 bytes = 256 bits)
        const key = getRandomBytes(32);

        // Generate random IV (16 bytes = 128 bits)
        const iv = getRandomBytes(16);

        // Generate random salt (8 bytes)
        const salt = getRandomBytes(8);

        // Derive encryption key
        const derivedKey = await deriveKey(key, password, salt);

        // Build adata (authenticated data)
        const adata = [
            [
                arrayBufferToBase64(iv),
                arrayBufferToBase64(salt),
                ITERATIONS,
                KEY_SIZE,
                TAG_SIZE,
                ALGORITHM,
                MODE,
                'none' // compression (none for simplicity)
            ],
            'plaintext', // formatter
            0, // open discussion (will be updated)
            0  // burn after reading (will be updated)
        ];

        // Update adata with actual settings
        adata[2] = document.getElementById('open-discussion')?.checked ? 1 : 0;
        adata[3] = document.getElementById('burn-after-reading')?.checked ? 1 : 0;

        // Convert plaintext to bytes
        const plaintextBytes = stringToUint8Array(plaintext);

        // Encrypt with AES-GCM
        const ciphertext = await crypto.subtle.encrypt(
            {
                name: 'AES-GCM',
                iv: iv,
                additionalData: stringToUint8Array(JSON.stringify(adata)),
                tagLength: TAG_SIZE
            },
            derivedKey,
            plaintextBytes
        );

        return {
            key: key,
            ciphertext: arrayBufferToBase64(ciphertext),
            adata: adata
        };
    }

    /**
     * Decrypt data using AES-256-GCM
     */
    async function decrypt(ciphertext, adata, key, password) {
        // Parse adata to get encryption parameters
        const spec = adata[0];
        const iv = base64ToArrayBuffer(spec[0]);
        const salt = base64ToArrayBuffer(spec[1]);

        // Derive decryption key
        const derivedKey = await deriveKey(key, password, new Uint8Array(salt));

        // Decrypt
        const plaintext = await crypto.subtle.decrypt(
            {
                name: 'AES-GCM',
                iv: new Uint8Array(iv),
                additionalData: stringToUint8Array(JSON.stringify(adata)),
                tagLength: TAG_SIZE
            },
            derivedKey,
            base64ToArrayBuffer(ciphertext)
        );

        return uint8ArrayToString(new Uint8Array(plaintext));
    }

    // =====================
    // API Functions
    // =====================

    /**
     * Create a new paste
     */
    async function createPaste() {
        const content = document.getElementById('paste-content').value;
        if (!content.trim()) {
            showAlert('Please enter some content', 'error');
            return;
        }

        const password = document.getElementById('password').value;
        const expire = document.getElementById('expire').value;

        try {
            showAlert('Encrypting...', 'info');

            // Encrypt the content
            const encrypted = await encrypt(content, password);

            // Build request
            const request = {
                v: 2,
                ct: encrypted.ciphertext,
                adata: encrypted.adata,
                meta: {
                    expire: expire
                }
            };

            // Send to server
            const response = await fetch('/', {
                method: 'POST',
                headers: {
                    'Content-Type': 'application/json',
                    'X-Requested-With': 'JSONHttpRequest'
                },
                body: JSON.stringify(request)
            });

            const data = await response.json();

            if (data.status !== 0) {
                throw new Error(data.message || 'Failed to create paste');
            }

            // Store delete token in memory and sessionStorage for persistence
            deleteToken = data.deletetoken;
            if (deleteToken) {
                sessionStorage.setItem('deleteToken-' + data.id, deleteToken);
            }

            // Build URL with key in fragment
            const burnPrefix = encrypted.adata[3] === 1 ? '-' : '';
            const keyEncoded = base58Encode(encrypted.key);
            const newUrl = window.location.origin + '/?' + data.id + '#' + burnPrefix + keyEncoded;

            // Update URL and show success
            window.history.pushState({}, '', newUrl);

            showAlert('Paste created! URL copied to clipboard.', 'success');

            // Try to copy URL to clipboard
            try {
                await navigator.clipboard.writeText(newUrl);
            } catch (e) {
                console.log('Could not copy to clipboard');
            }

            // Reload to show paste view
            window.location.reload();

        } catch (error) {
            console.error('Create paste error:', error);
            showAlert('Error: ' + error.message, 'error');
        }
    }

    /**
     * Load a paste from the server
     */
    async function loadPaste(pasteId) {
        try {
            // Check for stored delete token from sessionStorage
            const storedToken = sessionStorage.getItem('deleteToken-' + pasteId);
            if (storedToken) {
                deleteToken = storedToken;
            }

            const response = await fetch('/?' + pasteId, {
                headers: {
                    'X-Requested-With': 'JSONHttpRequest'
                }
            });

            const data = await response.json();

            if (data.status !== 0) {
                showAlert(data.message || 'Paste not found', 'error');
                return;
            }

            currentPaste = data;

            // Check if burn-after-reading
            if (data.adata && data.adata[3] === 1) {
                document.getElementById('burn-warning').classList.remove('hidden');
                return;
            }

            // Try to decrypt
            await decryptPaste();

        } catch (error) {
            console.error('Load paste error:', error);
            showAlert('Error loading paste: ' + error.message, 'error');
        }
    }

    /**
     * Decrypt the current paste
     */
    async function decryptPaste(password) {
        if (!currentPaste) return;

        const key = getKeyFromUrl();
        if (!key || key.length === 0) {
            showAlert('No decryption key found in URL', 'error');
            return;
        }

        try {
            const plaintext = await decrypt(
                currentPaste.ct,
                currentPaste.adata,
                key,
                password || ''
            );

            // Show decrypted content
            document.getElementById('paste-text').textContent = plaintext;
            document.getElementById('paste-output').classList.remove('hidden');
            document.getElementById('password-prompt').classList.add('hidden');

            // Show paste info
            if (currentPaste.meta && currentPaste.meta.postdate) {
                const date = new Date(currentPaste.meta.postdate * 1000);
                document.getElementById('paste-date').textContent = 'Created: ' + date.toLocaleString();
            }

            // Show discussion if enabled
            if (currentPaste.meta && currentPaste.meta.opendiscussion) {
                document.getElementById('discussion').classList.remove('hidden');
                if (currentPaste.comments && currentPaste.comments.length > 0) {
                    renderComments(currentPaste.comments, key, password);
                }
            }

            // Enable clone button
            document.getElementById('clone-paste').classList.remove('hidden');

            // Show delete button if we have a delete token
            if (deleteToken) {
                document.getElementById('delete-paste').classList.remove('hidden');
            }

        } catch (error) {
            console.error('Decrypt error:', error);

            // If decryption fails, might need password
            if (!password) {
                document.getElementById('password-prompt').classList.remove('hidden');
                document.getElementById('paste-output').classList.add('hidden');
            } else {
                showAlert('Decryption failed. Wrong password?', 'error');
            }
        }
    }

    /**
     * Decrypt with password from prompt
     */
    async function decryptWithPassword() {
        const password = document.getElementById('decrypt-password').value;
        await decryptPaste(password);
    }

    /**
     * View burn-after-reading paste
     */
    async function viewBurnPaste() {
        document.getElementById('burn-warning').classList.add('hidden');
        await decryptPaste();
    }

    /**
     * Clone current paste into editor
     */
    function clonePaste() {
        const content = document.getElementById('paste-text').textContent;
        document.getElementById('paste-content').value = content;
        showCreateMode();
        document.getElementById('paste-content').focus();
    }

    /**
     * Copy paste URL to clipboard
     */
    async function copyUrl(e) {
        if (e) e.preventDefault();
        try {
            await navigator.clipboard.writeText(window.location.href);
            showAlert('URL copied to clipboard', 'success');
        } catch (error) {
            showAlert('Could not copy URL', 'error');
        }
    }

    /**
     * Delete the current paste
     */
    async function deletePaste() {
        if (!currentPaste || !deleteToken) {
            showAlert('Cannot delete: no delete token', 'error');
            return;
        }

        if (!confirm('Are you sure you want to delete this paste?')) {
            return;
        }

        try {
            const response = await fetch('/', {
                method: 'DELETE',
                headers: {
                    'Content-Type': 'application/json',
                    'X-Requested-With': 'JSONHttpRequest'
                },
                body: JSON.stringify({
                    pasteid: currentPaste.id,
                    deletetoken: deleteToken
                })
            });

            const data = await response.json();

            if (data.status !== 0) {
                throw new Error(data.message || 'Failed to delete paste');
            }

            // Clean up stored delete token
            sessionStorage.removeItem('deleteToken-' + currentPaste.id);

            showAlert('Paste deleted', 'success');
            setTimeout(() => {
                window.location.href = '/';
            }, 1500);

        } catch (error) {
            showAlert('Error: ' + error.message, 'error');
        }
    }

    /**
     * Add a comment to the paste
     */
    async function addComment() {
        const content = document.getElementById('comment-content').value;
        if (!content.trim()) {
            showAlert('Please enter a comment', 'error');
            return;
        }

        const key = getKeyFromUrl();
        const password = document.getElementById('decrypt-password')?.value || '';

        try {
            // Encrypt comment
            const encrypted = await encryptComment(content, key, password);

            const response = await fetch('/', {
                method: 'POST',
                headers: {
                    'Content-Type': 'application/json',
                    'X-Requested-With': 'JSONHttpRequest'
                },
                body: JSON.stringify({
                    pasteid: currentPaste.id,
                    parentid: currentPaste.id,
                    data: encrypted.ciphertext,
                    adata: encrypted.adata,
                    v: 2
                })
            });

            const data = await response.json();

            if (data.status !== 0) {
                throw new Error(data.message || 'Failed to add comment');
            }

            showAlert('Comment added', 'success');
            document.getElementById('comment-content').value = '';

            // Reload to show new comment
            window.location.reload();

        } catch (error) {
            showAlert('Error: ' + error.message, 'error');
        }
    }

    /**
     * Encrypt a comment (uses same nested adata format as pastes for compatibility)
     */
    async function encryptComment(plaintext, key, password) {
        const iv = getRandomBytes(16);
        const salt = getRandomBytes(8);

        const derivedKey = await deriveKey(key, password, salt);

        // Use nested adata format to match paste encryption structure
        // Format: [[iv, salt, iterations, keysize, tagsize, algo, mode, compression], format, opendiscussion, burnafterreading]
        const spec = [
            arrayBufferToBase64(iv),
            arrayBufferToBase64(salt),
            ITERATIONS,
            KEY_SIZE,
            TAG_SIZE,
            ALGORITHM,
            MODE,
            'none'  // no compression for comments
        ];
        const adata = [spec, 'plaintext', 0, 0];

        const ciphertext = await crypto.subtle.encrypt(
            {
                name: 'AES-GCM',
                iv: iv,
                additionalData: stringToUint8Array(JSON.stringify(adata)),
                tagLength: TAG_SIZE
            },
            derivedKey,
            stringToUint8Array(plaintext)
        );

        return {
            ciphertext: arrayBufferToBase64(ciphertext),
            adata: adata
        };
    }

    /**
     * Render comments
     */
    async function renderComments(comments, key, password) {
        const container = document.getElementById('comments');
        container.innerHTML = '';

        for (const comment of comments) {
            try {
                const plaintext = await decrypt(comment.data, comment.adata, key, password);

                const div = document.createElement('div');
                div.className = 'comment';

                const meta = document.createElement('div');
                meta.className = 'comment-meta';
                const date = new Date(comment.meta.postdate * 1000);
                meta.textContent = date.toLocaleString();

                const content = document.createElement('div');
                content.className = 'comment-content';
                content.textContent = plaintext;

                div.appendChild(meta);
                div.appendChild(content);
                container.appendChild(div);
            } catch (e) {
                console.error('Failed to decrypt comment:', e);
            }
        }
    }

    /**
     * Show alert message
     */
    function showAlert(message, type) {
        const alert = document.getElementById('alert');
        alert.textContent = message;
        alert.className = 'alert alert-' + type;
        alert.classList.remove('hidden');

        if (type !== 'error') {
            setTimeout(() => {
                alert.classList.add('hidden');
            }, 3000);
        }
    }

    // Public API
    return {
        init: init
    };
})();

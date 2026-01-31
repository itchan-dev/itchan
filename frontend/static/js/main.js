// Click-to-Reply Functionality
function addReplyLink(textarea, threadId, messageId) {
    if (textarea) {
        const replyText = '>>' + threadId + '#' + messageId + '\n';
        textarea.value = textarea.value + replyText;
        textarea.focus();
        textarea.setSelectionRange(textarea.value.length, textarea.value.length);
    }
}

// Message Preview System
class MessagePreview {
    constructor() {
        this.cache = new Map();
        this.maxCacheSize = 500;
        this.hoverTimeout = null;
        this.hideSuccessorsTimeout = null;
        this.hideAllTimeout = null;
        this.previewIndexes = new Map();
        this.previewHistory = [];
        this.navigationTimeout = 700;
        this.rootKey = null;
        this.init();
    }

    init() {
        this.setupEventListeners();
    }

    setupEventListeners() {
        document.addEventListener('mouseover', (e) => {
            if (e.target.classList.contains('message-link-preview')) {
                this.handleLinkMouseOver(e.target);
            } else if (e.target.closest('.message-preview')) {
                this.handlePreviewMouseOver(e.target.closest('.message-preview'));
            } else if (e.target.closest('.popup-reply-container')) {
                this.handleReplyMouseOver();
            }
        });

        document.addEventListener('mouseout', (e) => {
            if (e.target.classList.contains('message-link-preview') || e.target.closest('.message-preview') || e.target.closest('.popup-reply-container')) {
                this.handleChainMouseOut();
            }
        });

        document.addEventListener('click', (e) => {
            let preview = e.target.closest('.message-preview');
            if (preview) {
                this.hideAllSuccessors(preview.dataset.key);
            } else if (!e.target.closest('.message-link-preview, .popup-reply-container')) {
                this.hideAllPreviews();
            }
        });
    }

    // Handles hovering a LINK.
    handleLinkMouseOver(linkElement) {
        // The user is actively navigating, so cancel ALL pending hide actions.
        if (this.hideSuccessorsTimeout) clearTimeout(this.hideSuccessorsTimeout);
        if (this.hideAllTimeout) clearTimeout(this.hideAllTimeout);
        if (this.hoverTimeout) clearTimeout(this.hoverTimeout);

        // For creation-only we want lower timeout than for pruning and creation
        const noChain = this.previewHistory.length == 0;
        const parentPreview = linkElement.closest('.message-preview');
        const isLast = !noChain && parentPreview && (this.previewHistory.at(-1).dataset.key == this.getKey(parentPreview));
        if (noChain || isLast) {
            // If there are no chain, or we are navigating in a last chain element - there are no pruning
            this.hoverTimeout = setTimeout(() => {
                this.createPreview(linkElement);
            }, 150);
        } else {
            this.hoverTimeout = setTimeout(() => {
                this.pruneAndCreatePreview(linkElement);
            }, this.navigationTimeout);
        } 
    }

    // Handles hovering a PREVIEW.
    handlePreviewMouseOver(previewElement) {
        // The user is inside the chain, so cancel the master "hide everything" timer.
        if (this.hideAllTimeout) clearTimeout(this.hideAllTimeout);
        // Also cancel any pending successor hides, in case they moved quickly between previews.
        if (this.hideSuccessorsTimeout) clearTimeout(this.hideSuccessorsTimeout);

        // RULE #1: Delay hiding successors to forgive accidental hovers.
        this.hideSuccessorsTimeout = setTimeout(() => {
            this.hideAllSuccessors(previewElement.dataset.key);
        }, this.navigationTimeout); // 700ms delay gives the user time to correct their mouse movement.
    }

    handleReplyMouseOver() {
        // Remove hide timers, so user can type his reply looking at reply chain
        if (this.hideAllTimeout) clearTimeout(this.hideAllTimeout);
        if (this.hideSuccessorsTimeout) clearTimeout(this.hideSuccessorsTimeout);
    }

    // Handles leaving ANY element in the chain.
    handleChainMouseOut() {
        // The user is leaving, so cancel any action that would SHOW a new preview.
        if (this.hoverTimeout) clearTimeout(this.hoverTimeout);

        // Delete whole chain at once. We dont want successors to be hidden first
        if (this.hideSuccessorsTimeout) clearTimeout(this.hideSuccessorsTimeout);
        
        // Set the master timer to hide the entire chain if the user doesn't re-enter.
        this.hideAllTimeout = setTimeout(() => {
            this.hideAllPreviews();
        }, this.navigationTimeout);
    }

    async createPreview(linkElement) {
        const key = this.getKey(linkElement);
        const parentPost = linkElement.closest('.post');
        const parentPostKey = this.getKey(parentPost);
        if (!key || !parentPostKey) {
            console.debug(`Can't get key element. This should not happen.`);
            return
        };
        linkElement.dataset.key = key;
        linkElement.dataset.postKey = parentPostKey;
        if (!this.rootKey) {
            this.rootKey = parentPostKey;
        }
        // Dont show root message preview to prevent cycles
        if (key == this.rootKey) {
            return;
        }
        // Dont show preview on parent message itself
        if (parentPostKey == key) {
            return;
        }
        // Dont show preview that is already displayed
        if (this.previewIndexes.has(key)) {
            return;
        }
        // Check cache first
        let messageData = this.cache.get(key);
        if (messageData) {
            this.showPreview(linkElement, messageData);
            return;
        }
        // Fetch from API
        try {
            messageData = await this.fetchMessage(key);
            this.cacheMessage(key, messageData);
            this.showPreview(linkElement, messageData);
        } catch (error) {
            console.error('Failed to fetch message:', error);
        }
    }

    async pruneAndCreatePreview(linkElement) {
        // Cleanup current previews first
        // Branching/multiple chains isnt allowed
        this.prunePreviews(linkElement);
        await this.createPreview(linkElement);
    }

    prunePreviews(linkElement) {
        // Clear active previews to prevent branching and multiple chains
        const NO_PARENT_PREVIEW = -100;

        let parent = linkElement.closest('.message-preview');
        let parentKey;
        let parentIdx;
        if (parent) {
            parentKey = this.getKey(parent);
            parentIdx = this.previewIndexes.get(parentKey);
        } else {
            // Means we are ouside of preview chain
            // If we navigating root element, mark it -1
            // Otherwise -100 and always create new chain
            parent = linkElement.closest('.post');
            parentKey = this.getKey(parent);
            parentIdx = NO_PARENT_PREVIEW;
            if (parentKey == this.rootKey) {
                parentIdx = -1;
            }
        }

        const linkKey = this.getKey(linkElement);
        let linkIdx = -1;
        if (this.previewIndexes.has(linkKey)) {
            linkIdx = this.previewIndexes.get(linkKey);
        }
        if (linkIdx == (parentIdx + 1)) {
            // If closing and reopening preview wouldnt change anything
            // i.e. we will prune next element and instantly reopen it
            this.hideAllSuccessors(linkKey);
            return
        } else if (parentIdx >= 0) {
            this.hideAllSuccessors(parentKey);
        } else {
            // If we are not in the chain and our is not 1st element of the chain - start new chain
            this.hideAllPreviews();
        }
    }

    showPreview(linkElement, htmlContent) {
        const key = linkElement.dataset.key;

        // Parse HTML string into DOM element
        const tempDiv = document.createElement('div');
        tempDiv.innerHTML = htmlContent;
        const preview = tempDiv.firstElementChild;

        if (!preview) {
            console.error('Failed to parse preview HTML');
            return;
        }

        preview.dataset.key = key;

        // We need this to get element size inside positionPreviewNearSource
        preview.style.visibility = 'hidden';
        preview.style.top = '0px';
        preview.style.left = '0px'
        document.body.appendChild(preview);

        this.positionPreviewNearSource(linkElement, preview);

        preview.style.visibility = 'visible';

        this.previewHistory.push(preview);
        this.previewIndexes.set(key, this.previewHistory.length - 1);
    }

    positionPreviewNearSource(linkElement, preview) {
        // We are trying to position our preview towards furthest quarter of a page
        const rect = linkElement.getBoundingClientRect();
    
        const previewWidth = preview.offsetWidth;
        const previewHeight = preview.offsetHeight;
    
        // If we are closer to the right side of the screen - position preview towards left side
        let left = window.scrollX + rect.right + 5;
        if (rect.right > (window.innerWidth / 2)) {
            left = window.scrollX + rect.left + 5 - previewWidth;
        }

         // If we are closer to the bottom of the screen - position preview towards top
        let top = window.scrollY + rect.top;
        if (rect.top > (window.innerHeight / 2)) {
            top = window.scrollY + rect.bottom - previewHeight;
        }

        preview.style.left = `${left}px`;
        preview.style.top = `${top}px`;
    }

    async fetchMessage(key) {
        const [board, threadId, messageId] = key.split('-');
        const response = await fetch(`/api/v1/${board}/${threadId}/${messageId}/html`, {
            method: 'GET',
            credentials: 'include'
        });

        if (!response.ok) {
            throw new Error(`HTTP ${response.status}: ${response.statusText}`);
        }
        return await response.text();
    }

    cacheMessage(key, data) {
        if (this.cache.size >= this.maxCacheSize) {
            const firstKey = this.cache.keys().next().value;
            this.cache.delete(firstKey);
        }
        this.cache.set(key, data);
    }

    hideAllPreviews() {
        this.previewHistory.forEach(preview => {
            if (preview && preview.parentNode) {
                preview.parentNode.removeChild(preview);
            }
        });

        // Reset state
        this.previewHistory = [];
        this.previewIndexes.clear();
        this.rootKey = null;
    }

    hideAllSuccessors(key) {
        if (!this.previewIndexes.has(key)) return;
        
        const idx = this.previewIndexes.get(key);
        
        // Get all previews that came after the current one
        const successors = this.previewHistory.slice(idx + 1);

        successors.forEach(preview => {
            if (preview && preview.parentNode) {
                preview.parentNode.removeChild(preview);
                this.previewIndexes.delete(preview.dataset.key);
            }
        });

        this.previewHistory.splice(idx + 1);
    }

    getKey(element) {
        if (!element) {
            return null;
        }
        const messageId = element.dataset.messageId;
        const threadId = element.dataset.threadId;
        const board = element.dataset.board;

        if (!messageId || !threadId || !board) {
            return null; // Return null for invalid links
        }
        return `${board}-${threadId}-${messageId}`
    }
}

function setupPopupReplySystem() {
    const popup = document.querySelector('.popup-reply-container');
    if (!popup) return; // Do nothing if the popup isn't on the page

    // Main click listener using event delegation
    document.body.addEventListener('click', (e) => {
        const replyLink = e.target.closest('.post-reply-popup-link');

        // If we clicked a reply link...
        if (replyLink) {
            e.preventDefault();

            const postElement = replyLink.closest('.post');
            if (!postElement) return;

            const board = postElement.dataset.board;
            const threadId = postElement.dataset.threadId;
            const messageId = postElement.dataset.messageId;

            // 1. CONFIGURE: Update the form's action attribute for current board/thread
            const form = popup.querySelector('form');
            form.action = `/${board}/${threadId}`;

            // 2. POSITION: Place it near the clicked link
            const linkRect = replyLink.getBoundingClientRect();
            popup.style.top = `${window.scrollY + linkRect.bottom + 5}px`;
            popup.style.left = `${window.scrollX + linkRect.left}px`;
            popup.style.display = 'block';

            // 3. ACTIVATE: Add reply link to the textarea (accumulates)
            const textarea = popup.querySelector('textarea');
            addReplyLink(textarea, threadId, messageId);

            // Note: File preview works automatically via event delegation - no setup needed!

            // Stop the click from closing the form immediately (see below)
            e.stopPropagation();
            return;
        }

        // Logic to close the popup
        // If the click was on the close button...
        if (e.target.closest('.popup-close-btn')) {
            e.stopPropagation(); // Prevent event from bubbling to MessagePreview handler
            popup.style.display = 'none';
            return;
        }

        // If the click was anywhere on the body BUT not inside a popup...
        if (!e.target.closest('.popup-reply-container')) {
            popup.style.display = 'none';
        }
    });
}


// File Preview and Management System
class UploadPreviewManager {
    // Constants for configuration attributes
    static ATTR_MAX_FILES = 'data-max-files';
    static ATTR_MAX_TOTAL_SIZE = 'data-max-total-size';
    static ATTR_MAX_FILE_SIZE = 'data-max-file-size';
    static ERROR_DISPLAY_DURATION = 5000; // ms

    constructor() {
        this.fileIdCounter = 0; // Global counter for unique file IDs
        this.fileMaps = new Map(); // Map of input -> Map<fileId, file>
        this.previewURLs = new Map(); // Map of input -> Map<fileId, objectURL> for cleanup
        this.init();
    }

    init() {
        // Single delegated event listener for ALL file inputs (existing + future)
        document.addEventListener('change', (e) => {
            const input = e.target;
            if (input.matches('input[type="file"][multiple]')) {
                const previewContainer = input.parentElement.querySelector('.file-preview-list');
                if (previewContainer) {
                    this.ensureInputInitialized(input);
                    this.handleFileSelection(input, previewContainer);
                }
            }
        });
    }

    ensureInputInitialized(input) {
        // Lazy initialization - only set up storage when first needed
        if (!this.previewURLs.has(input)) {
            this.previewURLs.set(input, new Map());
            this.fileMaps.set(input, new Map());
        }
    }

    handleFileSelection(input, previewContainer) {
        const files = Array.from(input.files);
        const maxFiles = parseInt(input.getAttribute(UploadPreviewManager.ATTR_MAX_FILES)) || 0;
        const maxTotalSize = parseInt(input.getAttribute(UploadPreviewManager.ATTR_MAX_TOTAL_SIZE)) || 0;
        const maxFileSize = parseInt(input.getAttribute(UploadPreviewManager.ATTR_MAX_FILE_SIZE)) || 0;

        // Validate max files limit
        if (maxFiles > 0 && files.length > maxFiles) {
            this.showValidationError(
                previewContainer,
                `You can only upload a maximum of ${maxFiles} file(s). Please select fewer files.`
            );
            input.value = '';
            return;
        }

        // Validate file sizes in a single pass (more efficient)
        if (maxFileSize > 0 || maxTotalSize > 0) {
            let totalSize = 0;
            const oversizedFiles = [];

            for (const file of files) {
                totalSize += file.size;
                if (maxFileSize > 0 && file.size > maxFileSize) {
                    oversizedFiles.push(file);
                }
            }

            // Check individual file size limit
            if (oversizedFiles.length > 0) {
                const maxSizeMB = (maxFileSize / (1024 * 1024)).toFixed(1);
                const fileList = oversizedFiles.map(f => `"${f.name}" (${this.formatFileSize(f.size)})`).join(', ');
                this.showValidationError(
                    previewContainer,
                    `The following file(s) exceed the ${maxSizeMB} MB limit: ${fileList}`
                );
                input.value = '';
                return;
            }

            // Check total size limit
            if (maxTotalSize > 0 && totalSize > maxTotalSize) {
                const maxSizeMB = (maxTotalSize / (1024 * 1024)).toFixed(1);
                const currentSizeMB = (totalSize / (1024 * 1024)).toFixed(2);
                this.showValidationError(
                    previewContainer,
                    `Total file size (${currentSizeMB} MB) exceeds the ${maxSizeMB} MB limit. Please select smaller files.`
                );
                input.value = '';
                return;
            }
        }

        // Clear any existing error messages
        this.clearValidationError(previewContainer);

        // Cleanup old object URLs before storing new files
        this.cleanupObjectURLs(input);

        // Store files with unique IDs in a Map
        const fileMap = new Map();
        files.forEach(file => {
            fileMap.set(this.fileIdCounter++, file);
        });
        this.fileMaps.set(input, fileMap);

        // Update preview (full rebuild on initial selection)
        this.buildPreview(input, previewContainer);
    }

    buildPreview(input, previewContainer) {
        const fileMap = this.fileMaps.get(input);
        if (!fileMap || fileMap.size === 0) {
            this.clearPreview(input, previewContainer);
            return;
        }

        const urlMap = this.previewURLs.get(input);

        // Clear existing preview
        previewContainer.innerHTML = '';

        // Add header
        const header = document.createElement('div');
        header.className = 'file-preview-header';
        header.textContent = `${fileMap.size} file(s) selected:`;
        previewContainer.appendChild(header);

        // Create preview items (Map maintains insertion order)
        fileMap.forEach((file, fileId) => {
            const fileItem = this.createFilePreviewItem(fileId, file, input, previewContainer, urlMap);
            previewContainer.appendChild(fileItem);
        });
    }

    createFilePreviewItem(fileId, file, input, previewContainer, urlMap) {
        const fileItem = document.createElement('div');
        fileItem.className = 'file-preview-item';
        fileItem.dataset.fileId = fileId;

        // Create thumbnail for images using Object URLs (much faster!)
        if (file.type.startsWith('image/')) {
            try {
                const objectURL = URL.createObjectURL(file);
                urlMap.set(fileId, objectURL);

                const img = document.createElement('img');
                img.src = objectURL;
                img.className = 'file-preview-thumbnail';
                img.onerror = () => {
                    // Fallback if image fails to load
                    img.style.display = 'none';
                    console.error(`Failed to load thumbnail for ${file.name}`);
                };
                fileItem.appendChild(img);
            } catch (error) {
                console.error(`Error creating object URL for ${file.name}:`, error);
            }
        } else if (file.type.startsWith('video/')) {
            const videoIcon = document.createElement('span');
            videoIcon.className = 'file-preview-icon';
            videoIcon.textContent = '🎬';
            fileItem.appendChild(videoIcon);
        }

        const fileName = document.createElement('span');
        fileName.className = 'file-preview-name';
        fileName.textContent = file.name;
        fileName.title = file.name;
        fileItem.appendChild(fileName);

        const fileSize = document.createElement('span');
        fileSize.className = 'file-preview-size';
        fileSize.textContent = this.formatFileSize(file.size);
        fileItem.appendChild(fileSize);

        const removeBtn = document.createElement('button');
        removeBtn.type = 'button';
        removeBtn.className = 'file-preview-remove';
        removeBtn.textContent = '×';
        removeBtn.title = 'Remove this file';
        removeBtn.addEventListener('click', (e) => {
            e.stopPropagation(); // Prevent event from bubbling to body listener
            this.removeFile(input, previewContainer, fileId);
        });
        fileItem.appendChild(removeBtn);

        return fileItem;
    }

    removeFile(input, previewContainer, fileId) {
        const fileMap = this.fileMaps.get(input);
        if (!fileMap || !fileMap.has(fileId)) return;

        const urlMap = this.previewURLs.get(input);

        // Cleanup object URL for the removed file
        const objectURL = urlMap.get(fileId);
        if (objectURL) {
            URL.revokeObjectURL(objectURL);
            urlMap.delete(fileId);
        }

        fileMap.delete(fileId);

        // Remove the DOM element (incremental update - no rebuild needed!)
        const fileItem = previewContainer.querySelector(`[data-file-id="${fileId}"]`);
        if (fileItem) {
            fileItem.remove();
        }

        // Update the header count
        const header = previewContainer.querySelector('.file-preview-header');
        if (header) {
            header.textContent = `${fileMap.size} file(s) selected:`;
        }

        // Update input.files with DataTransfer (Map maintains insertion order)
        const newDataTransfer = new DataTransfer();
        fileMap.forEach(file => newDataTransfer.items.add(file));
        input.files = newDataTransfer.files;

        // If no files left, clear the preview entirely
        if (fileMap.size === 0) {
            this.clearPreview(input, previewContainer);
        }
    }

    clearPreview(input, previewContainer) {
        this.cleanupObjectURLs(input);
        previewContainer.innerHTML = '';
        this.fileMaps.set(input, new Map());
    }

    cleanupObjectURLs(input) {
        const urlMap = this.previewURLs.get(input);
        if (urlMap) {
            // Revoke all object URLs to free memory
            urlMap.forEach(url => URL.revokeObjectURL(url));
            urlMap.clear();
        }
    }

    showValidationError(container, message) {
        this.clearValidationError(container);

        const errorDiv = document.createElement('div');
        errorDiv.className = 'file-validation-error';
        errorDiv.textContent = message;
        errorDiv.style.cssText = 'color: #d32f2f; padding: 8px; margin-bottom: 8px; background: #ffebee; border-radius: 4px; font-size: 14px;';

        container.parentElement.insertBefore(errorDiv, container);

        // Auto-hide after duration
        setTimeout(() => {
            this.clearValidationError(container);
        }, UploadPreviewManager.ERROR_DISPLAY_DURATION);
    }

    clearValidationError(container) {
        const errorDiv = container.parentElement.querySelector('.file-validation-error');
        if (errorDiv) {
            errorDiv.remove();
        }
    }

    formatFileSize(bytes) {
        if (bytes === 0) return '0 Bytes';
        const k = 1024;
        const sizes = ['Bytes', 'KB', 'MB', 'GB'];
        const i = Math.floor(Math.log(bytes) / Math.log(k));
        return Math.round(bytes / Math.pow(k, i) * 100) / 100 + ' ' + sizes[i];
    }

    // Cleanup method to prevent memory leaks (call when removing dynamic inputs)
    cleanup(input) {
        this.cleanupObjectURLs(input);
        this.fileMaps.delete(input);
        this.previewURLs.delete(input);
    }
}

// Handle reply hash for both page load and hash changes
function handleReplyHash() {
    const hash = window.location.hash;
    const replyMatch = hash.match(/^#reply-(\d+)$/);

    if (replyMatch) {
        const messageId = replyMatch[1];
        const textarea = document.getElementById('text-reply-bottom');

        if (textarea) {
            // Extract threadId from first post element
            const firstPost = document.querySelector('.post');
            if (firstPost) {
                const threadId = firstPost.dataset.threadId;
                addReplyLink(textarea, threadId, messageId);

                // Clean up URL: replace hash with textarea anchor
                history.replaceState(null, '', `${window.location.pathname}#text-reply-bottom`);
            }
        }
    }
}

document.addEventListener('DOMContentLoaded', () => {
    // Handle reply hash first (for board → thread redirects)
    handleReplyHash();

    window.messagePreview = new MessagePreview();
    console.log('Message preview system initialized');

    // Call the new, more powerful function
    setupPopupReplySystem();
    console.log('Popup reply system initialized');

    // Initialize file preview manager
    window.uploadPreviewManager = new UploadPreviewManager();
    console.log('Upload preview manager initialized');

    // Setup form confirmation handlers
    setupFormHandlers();

    // Add form validation for message posting (text OR attachments required)
    document.addEventListener('submit', (e) => {
        const form = e.target;

        // Check if this is a message posting form
        if (form.querySelector('textarea[name="text"]')) {
            if (!validateMessageForm(form)) {
                e.preventDefault();
                return false;
            }
        }
    });
});

// Handle hash changes on same page (thread page clicks)
window.addEventListener('hashchange', handleReplyHash);

// Setup form handlers (delete confirmations, blacklist prompts)
function setupFormHandlers() {
    // Handle all form submissions with event delegation
    document.addEventListener('submit', (e) => {
        const form = e.target;

        // Handle delete forms with confirmation
        if (form.classList.contains('js-confirm-form')) {
            const confirmMsg = form.dataset.confirmMessage || 'Are you sure you want to do this?';
            if (!confirm(confirmMsg)) {
                e.preventDefault();
                return false;
            }
        }

        // Handle blacklist forms with prompt
        if (form.classList.contains('blacklist-form')) {
            const reason = prompt("Enter reason for blacklist (optional):");
            if (reason === null) {
                // User clicked Cancel
                e.preventDefault();
                return false;
            }

            // Set the reason value in the hidden input
            const reasonInput = form.querySelector('input[name="reason"]');
            if (reasonInput) {
                reasonInput.value = reason || '';
            }
        }
    });
}

// Validation: Ensure message forms have either text OR attachments
function validateMessageForm(form) {
    const textField = form.querySelector('textarea[name="text"]');
    const fileInput = form.querySelector('input[type="file"][name="attachments"]');

    if (!textField && !fileInput) {
        return true; // Not a message form, allow submission
    }

    const hasText = textField && textField.value.trim().length > 0;
    const hasFiles = fileInput && fileInput.files && fileInput.files.length > 0;

    if (!hasText && !hasFiles) {
        alert('Please provide either text or attachments (or both) for your message.');
        return false;
    }

    return true;
}

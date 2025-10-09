// Click-to-Reply Functionality
function addReplyLink(textarea, threadId, messageId) {
    if (textarea) {
        const replyText = '>>' + threadId + '/' + messageId + '\n';
        textarea.value = textarea.value + replyText;
        textarea.focus();
        textarea.setSelectionRange(replyText.length, replyText.length);
    }
}

// Message Preview System
class MessagePreview {
    constructor() {
        this.template = document.getElementById('message-preview-template');
        if (!this.template) {
            console.error('Message preview template not found!');
        }

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
        if (this.template) {
            this.setupEventListeners();
        }
    }

    setupEventListeners() {
        document.addEventListener('mouseover', (e) => {
            if (e.target.classList.contains('message-link')) {
                this.handleLinkMouseOver(e.target);
            } else if (e.target.closest('.message-preview')) {
                this.handlePreviewMouseOver(e.target.closest('.message-preview'));
            } else if (e.target.closest('.popup-reply-container')) {
                this.handleReplyMouseOver();
            }
        });

        document.addEventListener('mouseout', (e) => {
            if (e.target.classList.contains('message-link') || e.target.closest('.message-preview') || e.target.closest('.popup-reply-container')) {
                this.handleChainMouseOut();
            }
        });

        document.addEventListener('click', (e) => {
            let preview = e.target.closest('.message-preview');
            if (preview) {
                this.hideAllSuccessors(preview.dataset.key);
            } else if (!e.target.closest('.message-link, .popup-reply-container')) {
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
            console.debug(`Can't get key element. Shit should not happen.`);
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

    showPreview(linkElement, messageData) {
        const key = linkElement.dataset.key;
        
        const clone = this.template.content.cloneNode(true);
        const preview = clone.querySelector('[data-js="preview-container"]');
        preview.dataset.key = key;

        this.renderPreviewContent(preview, messageData);

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

    renderPreviewContent(preview, messageData) {
        preview.dataset.board = messageData.Board;
        preview.dataset.messageId = messageData.Id;
        preview.dataset.threadId = messageData.ThreadId;

        // Find our placeholder elements using the data-js hooks
        const dateEl = preview.querySelector('[data-js="post-date"]');
        const linkEl = preview.querySelector('[data-js="post-link"]');
        const replyLinkEl = preview.querySelector('[data-js="post-reply-link"]');
        const repliesEl = preview.querySelector('[data-js="reply-links"]');
        const bodyEl = preview.querySelector('[data-js="post-body"]');
        const linkToMsg = `/${messageData.Board}/${messageData.ThreadId}#p${messageData.Id}`;

        dateEl.textContent = new Date(messageData.CreatedAt).toUTCString();
        linkEl.textContent = messageData.Id;
        linkEl.href = linkToMsg;
        replyLinkEl.href = linkToMsg;
        bodyEl.innerHTML = messageData.Text; // Safe because backend has escaped it

        repliesEl.innerHTML = '';
        if (messageData.Replies && messageData.Replies.length > 0) {
            messageData.Replies.forEach(reply => {
                const link = document.createElement('a');
                link.href = `/${messageData.Board}/${reply.FromThreadId}#p${reply.From}`;
                link.className = 'message-link'; // For hover previews
                link.dataset.board = messageData.Board;
                link.dataset.messageId = reply.From;
                link.dataset.threadId = reply.FromThreadId;
                link.textContent = `>>${reply.FromThreadId}/${reply.From}`;
                repliesEl.appendChild(link);
                repliesEl.appendChild(document.createTextNode(' ')); // For spacing
            });
        }
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
        const response = await fetch(`/api/v1/${board}/${threadId}/${messageId}`, {
            method: 'GET',
            credentials: 'include'
        });

        if (!response.ok) {
            throw new Error(`HTTP ${response.status}: ${response.statusText}`);
        }
        return await response.json();
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
    const template = document.getElementById('popup-reply-template');
    if (!template) return; // Do nothing if the template isn't on the page

    let currentPopup = null; // Keep track of the currently open popup

    // Function to remove the popup
    const removeCurrentPopup = () => {
        if (currentPopup) {
            currentPopup.remove();
            currentPopup = null;
        }
    };

    // Main click listener using event delegation
    document.body.addEventListener('click', (e) => {
        const replyLink = e.target.closest('.post-reply-popup-link');

        // If we clicked a reply link...
        if (replyLink) {
            e.preventDefault();

            // Remove any existing popup before creating a new one
            removeCurrentPopup();

            const postElement = replyLink.closest('.post');
            if (!postElement) return;

            const board = postElement.dataset.board;
            const threadId = postElement.dataset.threadId;
            const messageId = postElement.dataset.messageId;

            // 1. CLONE: Create a new popup from the template
            const clone = template.content.cloneNode(true);
            const newPopup = clone.querySelector('.popup-reply-container');
            
            // 2. CONFIGURE: Set the form's action attribute
            const form = newPopup.querySelector('form');
            form.action = `/${board}/${threadId}`;
            
            // 3. POSITION: Place it near the clicked link
            document.body.appendChild(newPopup);
            const linkRect = replyLink.getBoundingClientRect();
            newPopup.style.top = `${window.scrollY + linkRect.bottom + 5}px`;
            newPopup.style.left = `${window.scrollX + linkRect.left}px`;
            
            // 4. ACTIVATE: Call addReplyLink on the new textarea
            const textarea = newPopup.querySelector('textarea');
            addReplyLink(textarea, threadId, messageId);

            // Keep track of our new popup
            currentPopup = newPopup;

            // Stop the click from closing the form immediately (see below)
            e.stopPropagation();
            return;
        }
        
        // Logic to close the popup
        // If the click was on the close button...
        if (e.target.closest('.popup-close-btn')) {
            removeCurrentPopup();
            return;
        }

        // If the click was anywhere on the body BUT not inside a popup...
        if (currentPopup && !e.target.closest('.popup-reply-container')) {
            removeCurrentPopup();
        }

        if (e.target.closest('.post-reply-link')) {
            const postElement = e.target.closest('.post');
            if (!postElement) return;
            const threadId = postElement.dataset.threadId;
            const messageId = postElement.dataset.messageId;

            const textarea = document.getElementById('text-reply-bottom');
            addReplyLink(textarea, threadId, messageId);
        }
    });
}


document.addEventListener('DOMContentLoaded', () => {
    window.messagePreview = new MessagePreview();
    console.log('Message preview system initialized');
    
    // Call the new, more powerful function
    setupPopupReplySystem();
    console.log('Popup reply system initialized');
});

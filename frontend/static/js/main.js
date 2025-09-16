// Click-to-Reply Functionality
function addReplyLink(threadId, messageId) {
    const textarea = document.getElementById('text-reply-bottom');
    if (textarea) {
        const replyText = '>>' + threadId + '/' + messageId + '\n';
        textarea.value += replyText;
        textarea.focus();
        textarea.scrollTop = textarea.scrollHeight;
    }
}

// Message Preview System
class MessagePreview {
    constructor() {
        this.cache = new Map();
        this.maxCacheSize = 500;
        this.hoverTimeout = null;          // Timer to SHOW a new preview.
        this.hideSuccessorsTimeout = null; // Timer to HIDE successors (with a delay).
        this.hideAllTimeout = null;        // Master timer to HIDE the entire chain.
        this.previewIndexes = new Map();
        this.previewHistory = [];
        this.parentPost = null;            // This is not a preview, so it so not added to previewHistory
        this.init();
    }

    init() {
        this.setupEventListeners();
    }

    setupEventListeners() {
        document.addEventListener('mouseover', (e) => {
            if (e.target.classList.contains('message-link')) {
                this.handleLinkMouseOver(e.target);
            } else if (e.target.closest('.message-preview')) {
                this.handlePreviewMouseOver(e.target.closest('.message-preview'));
            }
        });

        document.addEventListener('mouseout', (e) => {
            if (e.target.classList.contains('message-link') || e.target.closest('.message-preview')) {
                this.handleChainMouseOut();
            }
        });

        document.addEventListener('click', (e) => {
            if (!e.target.closest('.message-link, .message-preview')) {
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

        // RULE #2: Links instantly destroy any successors from their parent.
        const parentPreview = linkElement.closest('.message-preview');
        if (parentPreview) {
            this.hideAllSuccessors(parentPreview.dataset.previewKey);
        } else {
            // If the link is not in a preview, it's a new chain. Clear everything.
            this.hideAllPreviews();
        }

        // Schedule the next preview to appear.
        this.hoverTimeout = setTimeout(() => {
            this.showPreview(linkElement);
        }, 300);
    }

    // Handles hovering a PREVIEW.
    handlePreviewMouseOver(previewElement) {
        // The user is inside the chain, so cancel the master "hide everything" timer.
        if (this.hideAllTimeout) clearTimeout(this.hideAllTimeout);
        // Also cancel any pending successor hides, in case they moved quickly between previews.
        if (this.hideSuccessorsTimeout) clearTimeout(this.hideSuccessorsTimeout);

        // RULE #1: Delay hiding successors to forgive accidental hovers.
        this.hideSuccessorsTimeout = setTimeout(() => {
            this.hideAllSuccessors(previewElement.dataset.previewKey);
        }, 700); // 700ms delay gives the user time to correct their mouse movement.
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
        }, 500); // A moderate delay before wiping the whole chain.
    }

    async showPreview(linkElement) {
        const previewKey = this.getPreviewKey(linkElement);
        if (!previewKey) return;
        // This means we are starting our chain, so we need to store parent
        if (this.previewHistory.length == 0) {  
            const parentPost = linkElement.closest('.post');
            // Should always be true
            if (parentPost) {
                this.parentPost = this.getPreviewKey(parentPost);
            }
        }

        if (previewKey == this.parentPost || this.previewIndexes.has(previewKey)) {
            return; // Already showing this preview in the current chain
        }

        // Check cache first
        let messageData = this.cache.get(previewKey);

        if (messageData) {
            this.createAndShowPreview(linkElement, messageData);
            return;
        }

        // Fetch from API
        try {
            messageData = await this.fetchMessage(previewKey);
            this.cacheMessage(previewKey, messageData);
            this.createAndShowPreview(linkElement, messageData);
        } catch (error) {
            console.error('Failed to fetch message:', error);
        }
    }
    
    async fetchMessage(previewKey) {
        const [board, threadId, messageId] = previewKey.split('-');
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

    createAndShowPreview(linkElement, messageData) {
        const previewKey = `${messageData.Board}-${messageData.ThreadId}-${messageData.Id}`;
        
        const previewIndex = this.previewHistory.length;

        const preview = document.createElement('div');
        preview.className = 'message-preview post';
        preview.dataset.previewKey = previewKey;

        this.renderPreviewContent(preview, messageData);
        this.positionPreviewNearSource(linkElement, preview);

        document.body.appendChild(preview);
        this.previewHistory.push(preview);
        this.previewIndexes.set(previewKey, previewIndex);
    }

    renderPreviewContent(preview, messageData) {
        const date = new Date(messageData.CreatedAt);
        const formattedDate = date.toLocaleString('en-US', { /* formatting options */ });

        let repliesHtml = '';
        if (messageData.Replies && messageData.Replies.length > 0) {
            repliesHtml = messageData.Replies.map(reply =>
                `<a href="/${messageData.Board}/${reply.FromThreadId}#p${reply.From}" class="message-link" data-board="${messageData.Board}" data-message-id="${reply.From}" data-thread-id="${reply.FromThreadId}">>>${reply.FromThreadId}/${reply.From}</a>`
            ).join(' ');
        }

        preview.innerHTML = `
            <div class="post-header">
                <span class="post-author">Anonymous</span>
                <span class="post-date">${formattedDate}</span>
                <span class="post-id">No.${messageData.Id}</span>
                ${repliesHtml ? `<span class="reply-links">${repliesHtml}</span>` : ''}
            </div>
            <div class="post-body">${messageData.Text}</div>
        `;
    }

    positionPreviewNearSource(linkElement, preview) {
        const rect = linkElement.getBoundingClientRect();
        preview.style.position = 'fixed';
        
        let left = rect.right + 10;
        let top = rect.top;

        // Simple positioning logic
        if (left + 400 > window.innerWidth) {
            left = rect.left - 410;
        }
        if (top + 200 > window.innerHeight) {
            top = window.innerHeight - 210;
        }

        preview.style.left = `${Math.max(10, left)}px`;
        preview.style.top = `${Math.max(10, top)}px`;
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
        this.parentPost = null;
    }

    hideAllSuccessors(previewKey) {
        if (!this.previewIndexes.has(previewKey)) return;
        
        const idx = this.previewIndexes.get(previewKey);
        
        // Get all previews that came after the current one
        const successors = this.previewHistory.slice(idx + 1);

        successors.forEach(preview => {
            if (preview && preview.parentNode) {
                preview.parentNode.removeChild(preview);
                this.previewIndexes.delete(preview.dataset.previewKey);
            }
        });

        this.previewHistory.splice(idx + 1);
    }

    getPreviewKey(element) {
        const messageId = element.dataset.messageId;
        const threadId = element.dataset.threadId;
        const board = element.dataset.board;

        if (!messageId || !threadId || !board) {
            return null; // Return null for invalid links
        }
        return `${board}-${threadId}-${messageId}`;
    }
}

document.addEventListener('DOMContentLoaded', () => {
    window.messagePreview = new MessagePreview();
    console.log('Message preview system initialized');
});

// Configuration
const MANAGER_URL = 'http://localhost:8080';
const DEFAULT_API_KEY = 'mk_admin_xyz789'; // Default API key for web UI
let currentPage = 1;
const pageLimit = 10;
let workers = new Map(); // Track running workers

// State mapping (Go integer states to human-readable strings)
const STATE_MAP = {
    0: 'Pending',
    1: 'Scheduled',
    2: 'Running',
    3: 'Completed',
    4: 'Failed'
};

// Helper function to get state name
function getStateName(state) {
    if (typeof state === 'number') {
        return STATE_MAP[state] || 'Unknown';
    }
    return state || 'Pending';
}

// Helper function to get state CSS class
function getStateClass(state) {
    const stateName = getStateName(state);
    return stateName.toLowerCase();
}

// Show notification
function showNotification(message, type = 'info') {
    const notification = document.getElementById('notification');
    notification.textContent = message;
    notification.className = `notification ${type} show`;
    
    setTimeout(() => {
        notification.classList.remove('show');
    }, 3000);
}

// Start a worker
async function startWorker() {
    const port = document.getElementById('workerPort').value;
    
    try {
        showNotification(`Starting worker on port ${port}...`, 'info');
        
        // In a real implementation, you'd make an API call to start the worker process
        // For now, we'll simulate it
        workers.set(port, {
            port: port,
            status: 'running',
            startedAt: new Date()
        });
        
        showNotification(`Worker started on port ${port}`, 'success');
        updateWorkersList();
        
        // Increment port for next worker
        document.getElementById('workerPort').value = parseInt(port) + 1;
    } catch (error) {
        showNotification(`Failed to start worker: ${error.message}`, 'error');
    }
}

// Stop a worker
async function stopWorker() {
    const port = document.getElementById('workerPort').value;
    
    try {
        if (!workers.has(port)) {
            showNotification(`No worker running on port ${port}`, 'error');
            return;
        }
        
        workers.delete(port);
        showNotification(`Worker on port ${port} stopped`, 'success');
        updateWorkersList();
    } catch (error) {
        showNotification(`Failed to stop worker: ${error.message}`, 'error');
    }
}

// Update workers list display
function updateWorkersList() {
    const container = document.getElementById('workersList');
    
    if (workers.size === 0) {
        container.innerHTML = '<p style="color: #718096;">No workers running. Start a worker to begin.</p>';
        return;
    }
    
    let html = '';
    workers.forEach((worker, port) => {
        const uptime = Math.floor((new Date() - worker.startedAt) / 1000);
        html += `
            <div class="worker-item active">
                <h4>Worker :${port}</h4>
                <div class="status running">● Running</div>
                <small style="color: #718096;">Uptime: ${uptime}s</small>
            </div>
        `;
    });
    
    container.innerHTML = html;
}

// Submit a task
async function submitTask(event) {
    event.preventDefault();
    
    const taskName = document.getElementById('taskName').value;
    const taskImage = document.getElementById('taskImage').value;
    const taskMemory = parseInt(document.getElementById('taskMemory').value);
    const taskDisk = parseInt(document.getElementById('taskDisk').value);
    const apiKey = document.getElementById('apiKey').value;
    
    const taskData = {
        task: {
            name: taskName,
            image: taskImage,
            memory: taskMemory,
            disk: taskDisk
        }
    };
    
    try {
        const response = await fetch(`${MANAGER_URL}/tasks`, {
            method: 'POST',
            headers: {
                'Content-Type': 'application/json',
                'Authorization': `Bearer ${apiKey}`
            },
            body: JSON.stringify(taskData)
        });
        
        if (!response.ok) {
            const error = await response.json();
            throw new Error(error.message || 'Failed to submit task');
        }
        
        const result = await response.json();
        showNotification(`Task "${taskName}" submitted successfully!`, 'success');
        
        // Clear form
        document.getElementById('taskForm').reset();
        document.getElementById('taskMemory').value = 128;
        document.getElementById('taskDisk').value = 1;
        document.getElementById('apiKey').value = DEFAULT_API_KEY;
        
        // Refresh tasks list
        setTimeout(refreshTasks, 500);
    } catch (error) {
        showNotification(`Error: ${error.message}`, 'error');
    }
}

// Fetch and display tasks
async function refreshTasks() {
    try {
        const response = await fetch(`${MANAGER_URL}/tasks?page=${currentPage}&limit=${pageLimit}`, {
            headers: {
                'Authorization': `Bearer ${DEFAULT_API_KEY}`
            }
        });
        
        if (!response.ok) {
            throw new Error('Failed to fetch tasks');
        }
        
        const data = await response.json();
        console.log('Received tasks data:', data); // Debug log
        displayTasks(data.tasks || []);
        updatePageInfo(data.pagination);
        updateSystemStatus(data);
    } catch (error) {
        console.error('Error in refreshTasks:', error); // Debug log
        showNotification(`Error fetching tasks: ${error.message}`, 'error');
        document.getElementById('tasksList').innerHTML = 
            '<p style="color: #718096;">Unable to load tasks. Make sure the manager is running.</p>';
    }
}

// Display tasks
function displayTasks(tasks) {
    const container = document.getElementById('tasksList');
    
    if (!tasks || tasks.length === 0) {
        container.innerHTML = '<p style="color: #718096;">No tasks found.</p>';
        return;
    }
    
    let html = '';
    tasks.forEach(task => {
        const stateName = getStateName(task.state);
        const stateClass = getStateClass(task.state);
        
        html += `
            <div class="task-item">
                <div>
                    <div class="task-name">${task.name || 'Unnamed'}</div>
                    <div class="task-image">${task.image || 'N/A'}</div>
                </div>
                <div style="font-size: 12px; color: #718096;">
                    <div>Memory: ${task.memory || 0} MB</div>
                    <div>Disk: ${task.disk || 0} GB</div>
                </div>
                <div class="task-state ${stateClass}">
                    ${stateName}
                </div>
                <div style="font-size: 12px; color: #718096;">
                    ${task.ID ? `ID: ${task.ID.substring(0, 8)}...` : 'No ID'}
                </div>
                <div>
                    <button onclick="deleteTask('${task.ID}')" class="btn btn-danger btn-sm" 
                            ${!task.ID ? 'disabled' : ''}>
                        Delete
                    </button>
                </div>
            </div>
        `;
    });
    
    container.innerHTML = html;
}

// Delete a task
async function deleteTask(taskId) {
    if (!confirm('Are you sure you want to delete this task?')) {
        return;
    }
    
    try {
        const response = await fetch(`${MANAGER_URL}/tasks/${taskId}`, {
            method: 'DELETE',
            headers: {
                'Authorization': `Bearer ${DEFAULT_API_KEY}`
            }
        });
        
        if (!response.ok) {
            const error = await response.json();
            throw new Error(error.message || 'Failed to delete task');
        }
        
        showNotification('Task deleted successfully', 'success');
        refreshTasks();
    } catch (error) {
        showNotification(`Error: ${error.message}`, 'error');
    }
}

// Update pagination info
function updatePageInfo(pagination) {
    if (!pagination) {
        document.getElementById('pageInfo').textContent = `Page ${currentPage}`;
        return;
    }
    
    const totalPages = Math.ceil((pagination.total || 0) / pageLimit);
    document.getElementById('pageInfo').textContent = 
        `Page ${currentPage} of ${totalPages} (${pagination.total || 0} tasks)`;
}

// Navigation
function nextPage() {
    currentPage++;
    refreshTasks();
}

function previousPage() {
    if (currentPage > 1) {
        currentPage--;
        refreshTasks();
    }
}

// Update system status
function updateSystemStatus(data) {
    const container = document.getElementById('systemStatus');
    
    const pagination = data.pagination || {};
    const totalTasks = pagination.total || 0;
    
    // Calculate task states
    const tasks = data.tasks || [];
    const stateCount = {
        pending: 0,
        scheduled: 0,
        running: 0,
        completed: 0,
        failed: 0
    };
    
    tasks.forEach(task => {
        try {
            const stateName = getStateName(task.state).toLowerCase();
            if (stateCount.hasOwnProperty(stateName)) {
                stateCount[stateName]++;
            }
        } catch (err) {
            console.error('Error processing task state:', task, err);
        }
    });
    
    container.innerHTML = `
        <div class="status-item">
            <h4>Total Tasks</h4>
            <div class="value">${totalTasks}</div>
        </div>
        <div class="status-item">
            <h4>Active Workers</h4>
            <div class="value">${workers.size}</div>
        </div>
        <div class="status-item">
            <h4>Running Tasks</h4>
            <div class="value">${stateCount.running}</div>
        </div>
        <div class="status-item">
            <h4>Completed Tasks</h4>
            <div class="value">${stateCount.completed}</div>
        </div>
    `;
}

// Initialize on page load
document.addEventListener('DOMContentLoaded', () => {
    updateWorkersList();
    refreshTasks();
    
    // Auto-refresh tasks every 5 seconds
    setInterval(refreshTasks, 5000);
});

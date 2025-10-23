# Minkube Web UI

A simple, clean web interface for managing your Minkube distributed container orchestration system.

## Features

### 🎛️ Worker Management
- Start and stop workers on different ports
- View active workers and their status
- Track worker uptime

### 📋 Task Management
- Submit new containerized tasks
- Configure memory and disk resources
- Monitor task status in real-time
- Delete tasks

### 📊 System Monitoring
- View total tasks across the cluster
- Track active workers
- Monitor running and completed tasks
- Auto-refresh every 5 seconds

## Quick Start

### 1. Start the Manager
```bash
# From the minkube root directory
make run-manager
```

The manager will start on `http://localhost:8080` and automatically serve the web UI.

### 2. Open the Web UI
Open your browser and navigate to:
```
http://localhost:8080
```

### 3. Start Workers (Optional - via CLI)
You can start workers through the Makefile or manually:
```bash
# Start 3 workers
make run-workers-dev

# Or start a single worker
make run-worker WORKER_PORT=8001
```

### 4. Submit Tasks
Use the web UI form to submit tasks, or use the API directly:
```bash
curl -X POST http://localhost:8080/tasks \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer admin-key-12345" \
  -d '{
    "task": {
      "name": "nginx-test",
      "image": "nginx:latest",
      "memory": 128,
      "disk": 1
    }
  }'
```

## API Keys

The web UI uses the default admin API key: `admin-key-12345`

Available keys (as configured in `manager/api.go`):
- `admin-key-12345` - Full admin access
- `dev-key-67890` - Developer access (read/write tasks)
- `readonly-key-11111` - Read-only access

## Development

### File Structure
```
web/
├── index.html   # Main HTML structure
├── styles.css   # Styling and responsive design
└── app.js       # JavaScript functionality and API integration
```

### Customization

**Change Manager URL:**
Edit `app.js` and update the `MANAGER_URL` constant:
```javascript
const MANAGER_URL = 'http://your-manager-host:8080';
```

**Auto-refresh Interval:**
Modify the interval in `app.js`:
```javascript
// Default: 5 seconds
setInterval(refreshTasks, 5000);
```

## Browser Support

- Chrome/Edge (latest)
- Firefox (latest)
- Safari (latest)

## Features Coming Soon

- [ ] Real worker process management from UI
- [ ] Task logs viewing
- [ ] Advanced filtering and search
- [ ] Task execution history
- [ ] Worker health metrics
- [ ] Dark mode theme
- [ ] Export tasks as JSON/CSV

## Troubleshooting

**UI not loading:**
- Ensure the manager is running: `make run-manager`
- Check the `web/` directory exists in your project root
- Verify no CORS issues in browser console

**Tasks not showing:**
- Check your API key is correct (`admin-key-12345` by default)
- Ensure manager can connect to workers
- Look at browser console for API errors

**Worker management not working:**
- Worker start/stop in UI is currently simulated for display
- Use Makefile commands for actual worker management
- Future versions will integrate real process management

## License

Same as Minkube project.

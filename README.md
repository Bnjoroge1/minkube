## Minkube Design Concept

Minkube is a distributed task management system. This document outlines its core design principles.

### Manager

The Manager is responsible for distributing tasks to available Worker nodes.

*   **Worker Selection:** It selects a Worker to receive a task using a round-robin strategy.
*   **Task Assignment:** It sends an HTTP request to the chosen Worker's API to assign the task and awaits a response.
*   **Failure Considerations:** The Manager is designed with the understanding that Workers might fail due to various reasons, such as:
    *   Failure to pull or use a specified Docker image.
    *   Bugs within the Docker container causing it to fail on startup.
    *   Insufficient disk space on the Worker node.

### Worker

Workers execute the tasks assigned by the Manager.

*   **State Management:** Each Worker stores its state, including the IDs of tasks assigned to it, in memory (primarily using a map data structure).
*   **Fault Tolerance:** This in-memory storage of task IDs aims to provide fault tolerance. If the Manager node fails, a Worker retains information about the tasks it is expected to perform.
*   **Performance:** Local storage of task IDs allows for faster access, as the Worker does not need to make a network round trip to the Manager to retrieve this information.
*   **Data Reconciliation:** Periodically, each Worker reconciles its local task database with the Manager's task database to ensure consistency.

### Planned Enhancements & System Details

#### Distributed Key-Value Store for Tasks
*   **TODO:** Implement a distributed key-value store for managing task data. This store will be shared between the Manager and Workers, aiming to improve data consistency, fault tolerance, and overall system scalability.

#### CPU Utilization Measurement
*   For measuring CPU utilization, the initial approach considered using Performance Monitoring Counters. However, due to unavailability in the target cloud VM environment, the implementation was switched to reading CPU statistics from the `/proc/stat` file.

#### Task Definition and Lifecycle
*   **(Placeholder for Task Details)**
    *   *This section will detail the structure of a task, its possible states, and how its lifecycle is managed within the system.*

#### Service Discovery
*   **Current Approach:** Workers currently register themselves with the Manager upon startup. This is achieved by the Worker sending an HTTP request to a registration endpoint on the Manager, which then adds the Worker to its list of available nodes.
*   **Alternative Approaches Considered:**
    *   Utilizing a dedicated service discovery tool (e.g., Consul).
    *   Implementing DNS-based service discovery, similar to how Kubernetes resolves service IP addresses.
*   **TODO:** Implement DNS-based service discovery as the preferred method for a more robust and scalable solution.
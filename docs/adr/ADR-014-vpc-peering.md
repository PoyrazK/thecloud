# ADR 014: VPC Peering Implementation (v2 - Cross-Tenant Support)

## Status
Accepted

## Context
As users deploy complex microservices across multiple isolated VPCs, there is a growing requirement for these services to communicate securely without traversing the public internet. Previously, VPCs were strictly isolated at the SDN layer using Open vSwitch (OVS) bridge boundaries.

We needed a solution that:
1.  **Ensures Security**: Restricts cross-VPC traffic to approved connections.
2.  **Prevents Routing Conflicts**: Handles overlapping IP address spaces (CIDR blocks).
3.  **Maintains Performance**: Leverages the existing SDN hardware/software offloading (OVS) rather than using high-latency user-space proxies.
4.  **Supports Cross-Tenant**: Allow peering between different tenants' VPCs with proper authentication.

## Decision
We implemented a VPC Peering service based on OVS flow-rule steering.

### 1. Peering Model & Life Cycle
We introduced a `VPCPeering` entity with a request-response flow:
-   **Pending Acceptance**: A requester VPC initiates a peering request to an accepter VPC in any tenant.
-   **Active**: Connectivity is established only after the owner (or authorized user in the tenant) accepts the request.
-   **OVS-Driven Data Plane**: When active, the system programs specific `priority=100` flow rules on both VPC bridges.

### 2. Cross-Bridge Routing (Data Plane)
Instead of physical patch cables or complex GRE/VXLAN tunnels for same-host peering, we utilize OVS flow rules:
-   **Flow Direction**: Traffic matching the peer VPC CIDR is targeted to the peer bridge.
-   **Isolation**: Only traffic matching the explicitly peered CIDR blocks is permitted to cross bridge boundaries.
-   **No-op Mode**: In development/Docker mode (where OVS might be simulated or absent), the peering status is updated in the database but network rules are skipped unless the OVS backend is enabled.

### 3. Safety Mechanisms
-   **CIDR Overlap Validation**: The service uses `net.IPNet` bitmasking to detect if two VPCs have overlapping IP ranges. Peering is rejected if overlaps exist to prevent routing ambiguity.
-   **VPC Deletion Guard**: A dependency check was added to `VpcService`. A VPC cannot be deleted if it has active peering connections, preventing "black hole" routes or orphaned SDN configurations.
-   **RBAC Integration**: New permissions (`vpc_peering:*`) ensure that only authorized users can manage connectivity.

### 4. Cross-Tenant Support (v2)
-   **Separate Tenant Tracking**: The `VPCPeering` entity tracks both `RequesterTenantID` and `AccepterTenantID` instead of a single `TenantID`.
-   **Cross-Tenant Queries**: Repository methods (`GetByID`, `List`, `UpdateStatus`, `Delete`) allow access from either tenant involved in the peering.
-   **Tenant Context Verification**: When creating a cross-tenant peering, the `AccepterTenantID` must be provided explicitly.

## Consequences

### Positive
-   **Low Latency**: Direct bridge-to-bridge routing via OVS kernel path.
-   **Scalability**: Avoids centralized routing bottlenecks.
-   **Improved UX**: Users can manage network topology via CLI/SDK with standard AWS/GCP-like workflows.
-   **Cross-Tenant Connectivity**: Different organizations can securely peer VPCs across tenant boundaries.

### Negative
-   **OVS Dependency**: The data-plane connectivity relies entirely on the OVS backend. If the backend is down or misconfigured, peering state in the DB may drift from the network state.
-   **CIDR Rigidity**: Once peered, VPC CIDR blocks cannot be changed easily without tearing down the peering first.

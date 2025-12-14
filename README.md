# tiaccoon
Tiaccoon achieve unified access control and container communication without dependence on specific transports by replacing the process of socket API.

## Implementation Roadmap
- [x] System call hooking
- [x] Transport selection
- [x] Access control
- [x] Notification of client's virtual address
- [x] RDMA support
- [ ] Communication with workload outside cluster
- [ ] CNI plugin
- [ ] Tiaccoond
- [ ] Integrate Tiaccoon Controller with Kubernetes

## Publications
- Slide(en): https://onoe.dev/middleware2025
- Slide(ja): https://onoe.dev/mthesis
- Hiroya Onoe, Daisuke Kotani, and Yasuo Okabe. Tiaccoon: Unified Access Control with Multiple Transports in Container Networks, MIDDLEWARE '25: Proceedings of the 26th International Middleware Conference, Vanderbilt University, Nashville, TN, USA, 14 December 2025.
  - https://doi.org/10.1145/3721462.3770783

# Environment - Mac UTM Debian

Requirements:

- ARM based Mac
- Homebrew and Ansible installed

## How to do a fresh start?

0. Clone this repo into your home folder `git clone https://github.com/dgf/Environment.git`

1. Run once the initial host setup `ansible-playbook host.yml`
2. Setup a new VM with UTM, mainly install a fresh Debian with a sudo user
3. Check in the new VM via `curl -D - -X POST http://192.168.64.1:7890/inventory -d"hostname=$(hostname)"`
3. Configure the Ansible inventory
4. Create or reuse a Playbook to provision that VM
5. SSH into it with `ssh user@$(./inventory/vms hostname)`


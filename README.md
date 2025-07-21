# solana-validator-failover

Simple p2p Solana validator failovers

A simple QUIC-based program to failover between Solana validators safely and quickly. [This post](https://blog.solstrategies.io/quic-solana-validator-failovers-738d712ac737) explains some background in more detail. In summary this program orchestrates the three-step process of failing over from an active (voting) to a passive (non-voting) validator:

1. active validator sets identity to passive
2. tower file synced from active to passive validator
3. passive validator sets identity to active

Start a failover server on the passive node:

![solanna-validator-failover-passive-to-active](vhs/failover-passive-to-active.gif)

Start a failover client on the active node to hand over to the passive node:

![solanna-validator-failover-active-to-passive](vhs/failover-active-to-passive.gif)

## Usage

```shell
# on any node declared in solana-validator-failover.yaml
# run the failover - a passive node will send a request to the active one to take over
# By default it runs in dry-run mode, to run for real, run on the passive node with `--not-a-drill`
solana-validator-failover run
```

By default, `run` runs in dry-run mode where only the tower file is synced between nodes and set identity commands are mocked. This is to safeguard against fat fingers (we've all been there) and also to give an idea of the expected total failover time under current network conditions.

⚠️ WARNING: _who_ you run this program as matters - the user:
- requires permissions to run set identity commands for the validator
- requires permissions to read/write the tower file - check inherited tower file permissions are what you expect after a dry-run

## Installation

Build from source or download the built package for your system from the [releases](https://github.com/SOL-Strategies/solana-validator-failover/releases) page. If your arch isn't listed, ping us.

## Prerequisites

1. A (_preferrably private_ and low-latency) UDP route between active and passive validators. Latency can vary lots across setups, so YMMV, though QUIC should give a good head start.
2. Some focus and appreciation of what you're doing - these can be high pucker factor operations regardless of tooling.

## Configuration

```yaml
# ~/solana-validator-failover/solana-validator-failover.yaml
validator:
  # path of validator program to use when issuing set-identity commands
  # default: agave-validator
  bin: agave-validator

  # cluster this validator runs on - one of: mainnet-beta, testnet, devnet, localnet
  cluster: mainnet-beta

  # this validator's identities
  identities:
    # path to solana-keygen file to use when ACTIVE
    active: /home/solana/active-validator-identity.json

    # path to solana-keygen file to use when PASSIVE
    passive: /home/solana/passive-validator-identity.json

  # ledger directory - made available to set-identity command templates
  ledger_dir: /mnt/ledger

  # local rpc address of this node
  # default: http://localhost:8899
  rpc_address: http://localhost:8899

  # tower file config
  tower:
    # directory hosting the tower file
    dir: /mnt/accounts/tower

    # when passive delete the towerfile if one exists before starting a failover server
    # default: false
    auto_empty_when_passive: false

    # golang template to identify the tower file within tower.dir
    # available to the template is an .Identities object
    # default: "tower-1_9-{{ .Identities.Active.PubKey }}.bin"
    file_name_template: "tower-1_9-{{ .Identities.Active.PubKey }}.bin"

  # failover configuration
  failover:

    # failover server config (runs on passive node taking over from active node)
    server:
      # default: 9898 - QUIC (udp) port to listen on
      port: 9898

    # golang template strings for command to set identity to active/passive
    # use this to set the appropriate command/args for your validator as required
    # available to this template will be:
    # {{ .Bin }}        - a resolved absolute path to the binary referenced in validator.bin
    # {{ .Identities }} - an object that has Active/Passive properties referencing
    #                     the loaded identities from validator.identities
    # {{ .LedgerDir }}  - a resolved absolute path to validator.ledger_dir
    # defaults shown below
    set_identity_active_cmd_template:  "{{ .Bin }} --ledger {{ .LedgerDir }} set-identity {{ .Identities.Active.KeyFile }} --require-tower"
    set_identity_passive_cmd_template: "{{ .Bin }} --ledger {{ .LedgerDir }} set-identity {{ .Identities.Passive.KeyFile }}"


    # failover peers - keys are vanity hostnames to help you review program output better
    peers:
      backup-validator-region-x:
        # host and port to connect to failover server
        address: backup-validator-region-x.some-private.zone:9898

    # (optional) hooks to run pre/post failover and when active or passive
    # the specified command program of a given hook will receive the following run-time env vars
    # it can choose to do what it wants to with:
    # IS_DRY_RUN_FAILOVER                     # true|false
    # THIS_NODE_ROLE                          # active|passive
    # THIS_NODE_NAME                          # hostname
    # THIS_NODE_PUBLIC_IP                     # pubic IP of node this program runs on
    # THIS_NODE_ACTIVE_IDENTITY_PUBKEY        # pubkey this node uses when active
    # THIS_NODE_ACTIVE_IDENTITY_KEYPAIR_FILE  # path to keyfile from validator.identities.active
    # THIS_NODE_PASSIVE_IDENTITY_PUBKEY       # pubkey this node uses when active
    # THIS_NODE_PASSIVE_IDENTITY_KEYPAIR_FILE # path to keyfile from validator.identities.active
    # THIS_NODE_CLIENT_VERSION                # gossip reported solana validator client semantic version for this node
    # PEER_NODE_ROLE                          # active|passive
    # PEER_NODE_NAME                          # hostname of peer
    # PEER_NODE_PUBLIC_IP                     # pubic IP of peer
    # PEER_NODE_ACTIVE_IDENTITY_PUBKEY        # pubkey peer uses when active
    # PEER_NODE_PASSIVE_IDENTITY_PUBKEY       # pubkey peer uses when passive
    # PEER_NODE_CLIENT_VERSION                # gossip reported solana validator client semantic version for peer node
    hooks:
      pre:
        # run before failover when validator is active
        when_active:
          - name: x # vanity name
            command: ./scripts/some_script.sh # command to run
            args: ["arg1", "arg2"]
            must_succeed: true # aborts failover on failure

        # run before failover when validator is passive
        when_passive:
          - name: x # vanity name
            command: ./scripts/some_script.sh # command to run
            args: ["arg1", "arg2"]
            must_succeed: true # aborts failover on failure

      # hooks to run after failover - errors in post hooks displayed but do nothing
      post:

        # run after failover when validator is active
        when_active:
          - name: x # vanity name
            command: ./scripts/some_script.sh # command to run
            args: ["arg1", "arg2"]
    
        # run after failover when validator is passive
        when_passive:
          - name: x # vanity name
            command: ./scripts/some_script.sh # command to run
            args: ["arg1", "arg2"]

    # duration string representing the minimum amount of time before the active node is due to
    # be the leader, if the failover is initiated below this threshold it will wait until this
    # window has passed to begin failing over
    # default: 5m
    min_time_to_leader_slot: 5m

    # post-failover monitoring config
    monitor:
      # monitoring of credit scroe and rank pre and post failover
      # to flag borked failovers showing stagnant credits or major rank slips
      credit_samples:
        # number of credit samples to take
        # default: 5
        count: 5

        # interval duration between samples
        # default: 5s
        interval: 5s
```

## Developing

```shell
# build in docker with live-reload on file changes
make dev
```

## Building

```shell
# build locally
make build

# or build from docker
make build-compose
```

## Laundry/wish list:

- [ ] Support forcing a failover for when active node is properly dead or similar situation
- [ ] Optionally skip tower file syncing (yolo)
- [ ] Set required user to run as and fail
- [ ] Automatic node discovery 
- [ ] TLS config
- [ ] Refactor to make e2e testing easier - current setup not optimal
- [ ] Optionally run as long-running service to support (almost) automatic failovers

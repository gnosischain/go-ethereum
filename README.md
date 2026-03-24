## Go Ethereum

This fork of go-ethereum runs the Golang execution layer implementation of the Ethereum protocol for Gnosis chains.

[![API Reference](
https://pkg.go.dev/badge/github.com/ethereum/go-ethereum
)](https://pkg.go.dev/github.com/ethereum/go-ethereum?tab=doc)
[![Go Report Card](https://goreportcard.com/badge/github.com/gnosischain/go-ethereum)](https://goreportcard.com/report/github.com/gnosischain/go-ethereum)
[![CI](https://github.com/gnosischain/go-ethereum/actions/workflows/go.yml/badge.svg?branch=master)](https://github.com/gnosischain/go-ethereum/actions/workflows/go.yml)
[![Discord](https://img.shields.io/badge/discord-join%20chat-blue.svg)](https://discord.gg/nthXNEv)
[![Twitter](https://img.shields.io/twitter/follow/go_ethereum)](https://x.com/go_ethereum)

## How This Fork Differs from Upstream

Gnosis differs from the standard Ethereum protocol in a few key ways. See [the specs](https://github.com/gnosischain/specs) for full details on the differences.

### Consensus

Pre-Merge:
- Gnosis runs AuRa as the consensus engine. The AuRa engine lives in [consensus/aura](./consensus/aura). The AuRa consensus engine still supports syncing from genesis and can process pre-merge blocks.

Post-Merge:
- Gnosis runs a beacon chain in the same way as Ethereum using `GNO` as the staking token. 
However, AuRa is still wrapped by `Beacon` in order to handle rewards and withdrwals with Gnosis-specific logic.

### Fees

Fees are **not** burned as the fee token is xDAI on Gnosis and will be collected to a contract and distributed rather than being burned.

### Syscalls and Service Transactions

Gnosis supports syscalls to system-level contracts defined here: [posdao-contracts](https://github.com/poanetwork/posdao-contracts/tree/master/contracts).

Service transactions are special transaction types on the Gnosis chain that can be submitted without paying for gas.

## Important Files

- [consensus/aura](./consensus/aura/): Contains the AuRa engine that provides pre-merge consensus and handles post-merge block rewards and withdrawals
- [core/state_processor.go](./core/state_processor.go): Handles AuRa syscalls and service transactions
- [params](./params/): Contains the gnosis [chainspecs](./params/chainspecs/) for Gnosis mainnet and Chiado testnet.


## Implementation

- [x] AuRa (Authority Round) consensus engine — full pre-merge validation + post-merge contract duties
- [x] [EIP-1559 modifications](https://github.com/gnosischain/specs/blob/master/network-upgrades/london.md) — base fees collected to contract instead of burned (xDAI is bridged)
- [x] [Post-merge POSDAO](https://github.com/gnosischain/specs/blob/master/execution/posdao-post-merge.md) — block rewards via contract syscall
- [x] [Gnosis withdrawals](https://github.com/gnosischain/specs/blob/master/execution/withdrawals.md) — routed through withdrawal contract
- [x] [EIP-4844-pectra](https://github.com/gnosischain/specs/blob/master/network-upgrades/pectra.md) — custom blob schedule (target 1, max 2)
- [x] Service transactions — zero-gas-price txs from certified senders
- [x] Bytecode rewriting — protocol-level contract code replacement (block-number and timestamp triggered)
- [x] Balancer hardfork
- [ ] Osaka (Chiado testnet timestamp set)


## Building the source

For prerequisites and detailed build instructions please read the [Installation Instructions](https://geth.ethereum.org/docs/getting-started/installing-geth).

Building `geth` requires both a Go (version 1.23 or later) and a C compiler. You can install
them using your favourite package manager. Once the dependencies are installed, run

```shell
make geth
```

or, to build the full suite of utilities:

```shell
make all
```

## Executables

The go-ethereum project comes with several wrappers/executables found in the `cmd`
directory.

|  Command   | Description                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                        |
| :--------: | ---------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| **`geth`** | Our main Ethereum CLI client. It is the entry point into the Ethereum network (main-, test- or private net), capable of running as a full node (default), archive node (retaining all historical state) or a light node (retrieving data live). It can be used by other processes as a gateway into the Ethereum network via JSON RPC endpoints exposed on top of HTTP, WebSocket and/or IPC transports. `geth --help` and the [CLI page](https://geth.ethereum.org/docs/fundamentals/command-line-options) for command line options. |
|   `clef`   | Stand-alone signing tool, which can be used as a backend signer for `geth`.                                                                                                                                                                                                                                                                                                                                                                                                                                                        |
|  `devp2p`  | Utilities to interact with nodes on the networking layer, without running a full blockchain.                                                                                                                                                                                                                                                                                                                                                                                                                                       |
|  `abigen`  | Source code generator to convert Ethereum contract definitions into easy-to-use, compile-time type-safe Go packages. It operates on plain [Ethereum contract ABIs](https://docs.soliditylang.org/en/develop/abi-spec.html) with expanded functionality if the contract bytecode is also available. However, it also accepts Solidity source files, making development much more streamlined. Please see our [Native DApps](https://geth.ethereum.org/docs/developers/dapp-developer/native-bindings) page for details.                                  |
|   `evm`    | Developer utility version of the EVM (Ethereum Virtual Machine) that is capable of running bytecode snippets within a configurable environment and execution mode. Its purpose is to allow isolated, fine-grained debugging of EVM opcodes (e.g. `evm --code 60ff60ff --debug run`).                                                                                                                                                                                                                                               |
| `rlpdump`  | Developer utility tool to convert binary RLP ([Recursive Length Prefix](https://ethereum.org/en/developers/docs/data-structures-and-encoding/rlp)) dumps (data encoding used by the Ethereum protocol both network as well as consensus wise) to user-friendlier hierarchical representation (e.g. `rlpdump --hex CE0183FFFFFFC4C304050583616263`).                                                                                                                                                                                |

## Running `geth`

Going through all the possible command line flags is out of scope here (please consult our
[CLI Wiki page](https://geth.ethereum.org/docs/fundamentals/command-line-options)),
but we've enumerated a few common parameter combos to get you up to speed quickly
on how you can run your own `geth` instance.

### Hardware Requirements

Minimum:

* CPU with 4+ cores
* 8GB RAM
* 1TB free storage space to sync the Mainnet
* 8 MBit/sec download Internet service

Recommended:

* Fast CPU with 8+ cores
* 16GB+ RAM
* High-performance SSD with at least 1TB of free space
* 25+ MBit/sec download Internet service

### Running a node

```shell
# Gnosis Mainnet
> geth --gnosis --authrpc.jwtsecret /path/to/jwt.hex

# Chiado Testnet
> geth --chiado --authrpc.jwtsecret /path/to/jwt.hex
```

For a more detailed walkthrough on running a full node of the Gnosis Chain, please refer to the [interactive guide](https://docs.gnosischain.com/node/manual/).

#### Docker quick start

One of the quickest ways to get a Gnosis node up and running on your machine is by using
Docker:

```shell
docker run -d --name gnosis-node -v /Users/alice/gnosis:/root \
           -p 8545:8545 -p 30303:30303 \
           ghcr.io/gnosischain/geth:latest --gnosis --authrpc.jwtsecret /root/jwt.hex
```

This will start `geth` in snap-sync mode on Gnosis mainnet. It will also create a persistent
volume in your home directory for saving your blockchain as well as map the default ports.

Do not forget `--http.addr 0.0.0.0`, if you want to access RPC from other containers
and/or hosts. By default, `geth` binds to the local interface and RPC endpoints are not
accessible from the outside.

## Contribution

Thank you for considering helping out with the source code! We welcome contributions
from anyone on the internet, and are grateful for even the smallest of fixes!

If you'd like to contribute to go-ethereum, please fork, fix, commit and send a pull request
for the maintainers to review and merge into the main code base. If you wish to submit
more complex changes though, please check up with the core devs first on [our Discord Server](https://discord.gg/invite/nthXNEv)
to ensure those changes are in line with the general philosophy of the project and/or get
some early feedback which can make both your efforts much lighter as well as our review
and merge procedures quick and simple.

Please make sure your contributions adhere to our coding guidelines:

 * Code must adhere to the official Go [formatting](https://golang.org/doc/effective_go.html#formatting)
   guidelines (i.e. uses [gofmt](https://golang.org/cmd/gofmt/)).
 * Code must be documented adhering to the official Go [commentary](https://golang.org/doc/effective_go.html#commentary)
   guidelines.
 * Pull requests need to be based on and opened against the `master` branch.
 * Commit messages should be prefixed with the package(s) they modify.
   * E.g. "eth, rpc: make trace configs optional"

Please see the [Developers' Guide](https://geth.ethereum.org/docs/developers/geth-developer/dev-guide)
for more details on configuring your environment, managing project dependencies, and
testing procedures.

### Contributing to geth.ethereum.org

For contributions to the [go-ethereum website](https://geth.ethereum.org), please checkout and raise pull requests against the `website` branch.
For more detailed instructions please see the `website` branch [README](https://github.com/ethereum/go-ethereum/tree/website#readme) or the 
[contributing](https://geth.ethereum.org/docs/developers/geth-developer/contributing) page of the website.

## License

The go-ethereum library (i.e. all code outside of the `cmd` directory) is licensed under the
[GNU Lesser General Public License v3.0](https://www.gnu.org/licenses/lgpl-3.0.en.html),
also included in our repository in the `COPYING.LESSER` file.

The go-ethereum binaries (i.e. all code inside of the `cmd` directory) are licensed under the
[GNU General Public License v3.0](https://www.gnu.org/licenses/gpl-3.0.en.html), also
included in our repository in the `COPYING` file.

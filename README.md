# Filetransfer

## Usage

Transfer files on a LAN between two peers.

Receiver:

```
./filetransfer receive DEST_DIRECTORY
```

Sender:

```
./filetransfer send FILENAME
```

## Tech

This tools uses plain TCP to transfer the files and mDNS for auto-discovery.

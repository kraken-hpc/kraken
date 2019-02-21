DEFAULT menu.c32
PROMPT 0
TIMEOUT 50
MENU TITLE Main Menu

LABEL node
    MENU LABEL Boot x86_64 Compute Node (diskless)
    KERNEL vmlinuz
    APPEND console=tty1 root=/dev/ram0 initrd=initramfs.cpio.gz kraken.ip={{.IP}} kraken.net={{.CIDR}} kraken.id={{.ID}} kraken.parent={{.ParentIP}}

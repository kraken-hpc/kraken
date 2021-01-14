#!/usr/bin/python3

from __future__ import (absolute_import, division, print_function)
__metaclass__ = type

ANSIBLE_METADATA = {
    'metadata_version': '1.0',
    'status': ['preview'],
    'supported_by': 'community'
}

import binascii
from ansible.errors import AnsibleFilterError
from base64 import b64encode, b64decode
from uuid import UUID
from socket import inet_aton, inet_ntoa
from netaddr import EUI, AddrFormatError

# convert a UUID to b64 encoded bytes
def uuid_to_b64(value):
    try:
        u = UUID(value)
    except ValueError as e:
        raise AnsibleFilterError('uuid_to_b64: (%s) is not a valid UUID: %s' % (value, str(e)))
    return b64encode(u.bytes).decode()

# convert b64 encoded bytes to a UUID
def b64_to_uuid(value):
    try:
        b = b64decode(value)
    except binascii.Error as e:
        raise AnsibleFilterError('b64_to_uuid: failed to decode (%s) as base64: %s' % (value, str(e)))
    try:
        u = UUID(bytes=b)
    except ValueError as e:
        raise AnsibleFilterError('b64_to_uuid: (%s) does not appear to be a valid UUID: %s' % (value, str(e)))
    return str(u)

# convert ip4 address to b64 encoded bytes
def ip_to_b64(value):
    try:
        i = inet_aton(value)
    except OSError as e:
        raise AnsibleFilterError('ip_to_b64: (%s) is not a valid IPv4 address: %s' % (value, str(e)))
    return b64encode(i).decode()

# convert b64 to ip address
def b64_to_ip(value):
    try:
        b = b64decode(value)
    except binascii.Error as e:
        raise AnsibleFilterError('b64_to_ip: failed to decode (%s) as base64: %s' % (value, str(e)))
    try:
        i = inet_ntoa(b)
    except OSError as e:
        raise AnsibleFilterError('b64_to_ip: (%s) is not a valid IPv4 address: %s' % (value, str(e)))
    return i

def mac_to_b64(value):
    try:
        m = EUI(value)
    except AddrFormatError as e:
        raise AnsibleFilterError('mac_to_b64: (%s) is not a valid MAC address: %s' % (value, str(e)))
    return b64encode(m.packed).decode()

def b64_to_mac(value):
    try:
        b = b64decode(value)
    except binascii.Error as e:
        raise AnsibleFilterError('b64_to_mac: failed to decode (%s) as base64: %s' % (value, str(e)))
    try:
        m = EUI(b.hex())
    except AddrFormatError as e:
        raise AnsibleFilterError('b64_to_mac: (%s) is not a valid MAC address: %s' % (value, str(e)))
    return m

# register ansible filters
class FilterModule(object):
    ''' b64-type filters '''

    def filters(self):
        return {
            'uuid_to_b64': uuid_to_b64,
            'b64_to_uuid': b64_to_uuid,
            'ip_to_b64': ip_to_b64,
            'b64_to_ip': b64_to_ip,
            'mac_to_b64': mac_to_b64,
            'b64_to_mac': b64_to_mac
        }
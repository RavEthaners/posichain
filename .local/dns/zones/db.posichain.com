$TTL    604800
@       IN      SOA     ns1.posichain.com. root.posichain.com. (
                  3       ; Serial
             604800     ; Refresh
              86400     ; Retry
            2419200     ; Expire
             604800 )   ; Negative Cache TTL
;
; name servers - NS records
     IN      NS      ns1.posichain.com.

; name servers - A records
ns1.posichain.com.                      IN      A      172.189.0.3

s0.z.d.posichain.com.                   IN      A      172.189.0.9  ; node 1
s0.z.d.posichain.com.                   IN      A      172.189.0.10 ; node 2
s0.z.d.posichain.com.                   IN      A      172.189.0.11 ; node 3
s0.z.d.posichain.com.                   IN      A      172.189.0.12 ; node 4
s0.z.d.posichain.com.                   IN      A      172.189.0.13 ; node 5
s1.z.d.posichain.com.                   IN      A      172.189.0.14 ; node 6
s1.z.d.posichain.com.                   IN      A      172.189.0.15 ; node 7
s1.z.d.posichain.com.                   IN      A      172.189.0.16 ; node 8
s1.z.d.posichain.com.                   IN      A      172.189.0.17 ; node 9
s1.z.d.posichain.com.                   IN      A      172.189.0.18 ; node 10
_dnsaddr.bootstrap.d.posichain.com.     IN      TXT     "dnsaddr=/ip4/172.189.0.8/tcp/9876/p2p/Qmc1V6W7BwX8Ugb42Ti8RnXF1rY5PF7nnZ6bKBryCgi6cv"

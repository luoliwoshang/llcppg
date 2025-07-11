// https://github.com/goplus/gogen/issues/496

struct ip6_addr
{
    int zone;
};

/** IPv6 address */
typedef struct ip6_addr ip6_addr_t;

typedef struct ip_addr
{
    union
    {
        ip6_addr_t ip6;
        ip4_addr_t ip4;
    } u_addr;
    /** @ref lwip_ip_addr_type */
    int type;
} ip_addr_t;

struct ip4_addr
{
    int addr;
};

/** ip4_addr_t uses a struct for convenience only, so that the same defines can
 * operate both on ip4_addr_t as well as on ip4_addr_p_t. */
typedef struct ip4_addr ip4_addr_t;

typedef union
{
    /** Args to LWIP_NSC_LINK_CHANGED callback */
    struct link_changed_s
    {
        /** 1: up; 0: down */
        int state;
    } link_changed;
    /** Args to LWIP_NSC_STATUS_CHANGED callback */
    struct status_changed_s
    {
        /** 1: up; 0: down */
        int state;
    } status_changed;
    /** Args to LWIP_NSC_IPV4_ADDRESS_CHANGED|LWIP_NSC_IPV4_GATEWAY_CHANGED|LWIP_NSC_IPV4_NETMASK_CHANGED|LWIP_NSC_IPV4_SETTINGS_CHANGED callback */
    struct ipv4_changed_s
    {
        /** Old IPv4 address */
        const ip_addr_t *old_address;
        const ip_addr_t *old_netmask;
        const ip_addr_t *old_gw;
    } ipv4_changed;
    /** Args to LWIP_NSC_IPV6_SET callback */
    struct ipv6_set_s
    {
        /** Index of changed IPv6 address */
        int addr_index;
        /** Old IPv6 address */
        const ip_addr_t *old_address;
    } ipv6_set;
    /** Args to LWIP_NSC_IPV6_ADDR_STATE_CHANGED callback */
    struct ipv6_addr_state_changed_s
    {
        /** Index of affected IPv6 address */
        int addr_index;
        /** Old IPv6 address state */
        int old_state;
        /** Affected IPv6 address */
        const ip_addr_t *address;
    } ipv6_addr_state_changed;
} netif_ext_callback_args_t;

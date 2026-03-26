/* MinZ stdalign.h for Z80 targets (C11) */
/* Z80 is byte-addressed — alignment is always 1 */
#ifndef _MINZ_STDALIGN_H
#define _MINZ_STDALIGN_H

#define alignas _Alignas
#define alignof _Alignof
#define __alignas_is_defined 1
#define __alignof_is_defined 1

#endif /* _MINZ_STDALIGN_H */

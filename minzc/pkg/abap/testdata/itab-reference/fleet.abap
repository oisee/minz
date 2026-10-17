REPORT zfleet.
TYPES: BEGIN OF ty_ship,
         name  TYPE c LENGTH 8,
         crew  TYPE i,
         cargo TYPE i,
       END OF ty_ship.
DATA ship TYPE ty_ship.
DATA other TYPE ty_ship.
DATA fleet TYPE STANDARD TABLE OF ty_ship WITH DEFAULT KEY INITIAL SIZE 4.
DATA total TYPE i.
DATA seen TYPE i.
DATA pairs TYPE i.
DATA lines_now TYPE i.
DATA described TYPE i.
DATA after TYPE i.

ship-name = 'AURORA'.
ship-crew = 12.
ship-cargo = 300.
APPEND ship TO fleet.
seen = sy-tabix.
ship-name = 'BOREAS'.
ship-crew = 7.
ship-cargo = -45.
APPEND ship TO fleet.
seen = seen * 10 + sy-tabix.
CLEAR ship.
ship-name = 'CALYPSO'.
ship-crew = 2147483.
APPEND ship TO fleet.
seen = seen * 10 + sy-tabix.

LOOP AT fleet INTO ship.
  total = total + ship-crew * sy-tabix.
  WRITE |{ ship-name }={ ship-crew };|.
ENDLOOP.
seen = seen * 10 + sy-subrc.

READ TABLE fleet INTO ship INDEX 2.
seen = seen * 10 + sy-subrc.
seen = seen * 10 + sy-tabix.
READ TABLE fleet INTO other INDEX 9.
seen = seen * 10 + sy-subrc.
seen = seen * 10 + sy-tabix.

lines_now = lines( fleet ).
DESCRIBE TABLE fleet LINES described.

LOOP AT fleet INTO ship.
  LOOP AT fleet INTO other.
    pairs = pairs + sy-tabix.
  ENDLOOP.
  pairs = pairs * 10 + sy-tabix.
ENDLOOP.
WRITE |{ ship-name }/{ other-cargo }|.

CLEAR fleet.
after = lines( fleet ).
LOOP AT fleet INTO other.
  after = 99.
ENDLOOP.
after = after * 10 + sy-subrc.
other-cargo = 5.
APPEND other TO fleet.
ship-name = 'DORADO-XL'.
APPEND ship TO fleet.
after = after * 10 + lines( fleet ).

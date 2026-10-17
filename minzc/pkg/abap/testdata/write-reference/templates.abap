REPORT zref.
DATA x TYPE i VALUE -65537.
DATA y TYPE c LENGTH 5 VALUE 'A'.
DATA seen TYPE i VALUE 7.
WRITE |<{ x }><{ y }>!|.
WRITE |a\nb\t\{c\}\|\\|.
WRITE |<{ x + 1 }>!|.

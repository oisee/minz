REPORT zsalv_semantic_excerpt.
TYPES: BEGIN OF ty_raw, text TYPE c LENGTH 80, END OF ty_raw.
TYPES: BEGIN OF ty_fleet, name TYPE c LENGTH 20, route TYPE c LENGTH 20, seats TYPE i, END OF ty_fleet.
DATA input TYPE STANDARD TABLE OF ty_raw INITIAL SIZE 2.
DATA raw TYPE ty_raw.
DATA fleet TYPE STANDARD TABLE OF ty_fleet INITIAL SIZE 4.
DATA flight TYPE ty_fleet.
DATA total TYPE i.
raw-text = 'Atlas,North,40'.
APPEND raw TO input.
raw-text = 'Beacon,South,32'.
APPEND raw TO input.
LOOP AT input INTO raw.
  PERFORM convert.
  APPEND flight TO fleet.
  total = total + flight-seats.
ENDLOOP.
WRITE |Total { total }|.
FORM convert.
  DATA seats_text TYPE c LENGTH 12.
  DATA seats TYPE i.
  SPLIT raw-text AT ',' INTO flight-name flight-route seats_text.
  seats = seats_text.
  flight-seats = seats.
ENDFORM.

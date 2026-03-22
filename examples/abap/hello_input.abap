REPORT zhello_input.

PARAMETERS: p_name  TYPE c LENGTH 20 DEFAULT 'World',
            p_count TYPE i DEFAULT 3.

START-OF-SELECTION.
  DO p_count TIMES.
    WRITE 'Hello, '.
    WRITE p_name.
    WRITE '!'.
  ENDDO.

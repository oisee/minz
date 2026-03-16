REPORT zhello_param.

PARAMETERS p_name TYPE c LENGTH 10 DEFAULT 'World'.

START-OF-SELECTION.
  WRITE 'Hello, '.
  WRITE p_name.
  WRITE '!'.

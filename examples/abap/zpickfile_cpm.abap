REPORT zpickfile_cpm.
TYPES: BEGIN OF ty_file,
         filename TYPE c LENGTH 12,
       END OF ty_file.
TYPES ty_line TYPE c LENGTH 128.
TYPES: BEGIN OF ty_flight,
         name TYPE c LENGTH 20,
         route TYPE c LENGTH 20,
         seats TYPE i,
       END OF ty_flight.
PARAMETERS p_file TYPE c LENGTH 12 DEFAULT 'EXTRA.CSV'.
DATA lt_files TYPE STANDARD TABLE OF ty_file WITH EMPTY KEY INITIAL SIZE 8.
DATA ls_file TYPE ty_file.
DATA lv_rc TYPE i.
DATA lv_action TYPE i.
DATA lt_lines TYPE STANDARD TABLE OF ty_line WITH EMPTY KEY INITIAL SIZE 16.
DATA line TYPE ty_line.
DATA fleet TYPE STANDARD TABLE OF ty_flight WITH EMPTY KEY INITIAL SIZE 16.
DATA flight TYPE ty_flight.
DATA seats_text TYPE c LENGTH 12.
DATA seats TYPE i.
DATA total TYPE i.
AT SELECTION-SCREEN ON VALUE-REQUEST FOR p_file.
  cl_gui_frontend_services=>file_open_dialog(
    EXPORTING window_title = 'Choose CSV' file_filter = '*.CSV'
              default_extension = 'CSV' initial_directory = ''
    CHANGING file_table = lt_files rc = lv_rc user_action = lv_action ).
  IF lv_rc > 0.
    READ TABLE lt_files INTO ls_file INDEX 1.
    p_file = ls_file-filename.
  ENDIF.
START-OF-SELECTION.
  cl_gui_frontend_services=>gui_upload(
    EXPORTING filename = p_file filetype = 'ASC'
    CHANGING data_tab = lt_lines
    EXCEPTIONS file_open_error = 1 OTHERS = 2 ).
  IF sy-subrc <> 0.
    WRITE / 'File not found'.
    RETURN.
  ENDIF.
  LOOP AT lt_lines INTO line.
    SPLIT line AT ',' INTO flight-name flight-route seats_text.
    seats = seats_text.
    flight-seats = seats.
    APPEND flight TO fleet.
    total = total + seats.
    WRITE / |{ flight-name } { flight-route } { flight-seats }|.
  ENDLOOP.
  WRITE / |Total seats: { total }|.

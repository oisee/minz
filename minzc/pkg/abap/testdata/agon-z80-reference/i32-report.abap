REPORT zagon32.
DATA low TYPE i VALUE -2147483648.
DATA high TYPE i VALUE 2147483647.
DATA wide TYPE i VALUE 65536.
DATA negative TYPE i VALUE -65537.
FORM combine USING amount TYPE i CHANGING total TYPE i.
  total = total + amount.
ENDFORM.
WRITE |MIN={ low };MAX={ high };|.
PERFORM combine USING wide CHANGING low.
high = high - wide.
wide = wide * 3 + 1.
negative = negative + wide.
WRITE |{ low };{ high };{ wide };{ negative }|.

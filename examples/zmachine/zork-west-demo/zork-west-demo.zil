"A small West of House demo adapted from MIT-licensed Zork I source."
<VERSION ZIP>
<CONSTANT RELEASEID 1>
<ROOM WEST-OF-HOUSE (DESC "West of House")>
<ROOM NORTH-OF-HOUSE (DESC "North of House")>
<ROOM EAST-OF-HOUSE (DESC "East of House")>
<ROOM SOUTH-OF-HOUSE (DESC "South of House")>
<GLOBAL HERE WEST-OF-HOUSE>
<GLOBAL SCORE 0>
<GLOBAL TURNS 0>
<GLOBAL BOX-OPEN <>>
<GLOBAL HAVE-LEAFLET <>>
<GLOBAL INBUF <ITABLE 80 (BYTE) 0>>
<GLOBAL LEXBUF <ITABLE 32 (BYTE) 0>>
<BUZZ LOOK L OPEN CLOSE MAILBOX BOX READ LEAFLET TAKE GET INVENTORY I HELP QUIT N NORTH S SOUTH E EAST W WEST GO THE A AN>
<ROUTINE GO () <MAIN>>
<ROUTINE DESCRIBE ()
  <COND
    (<EQUAL? ,HERE ,WEST-OF-HOUSE>
      <PRINTI "West of House|You are standing in an open field west of a white house, with a boarded front door. There is a small mailbox here.|">
      <COND (,BOX-OPEN <PRINTI "The mailbox is open.|">)>
      <COND (,HAVE-LEAFLET <PRINTI "You are carrying a leaflet.|">)>)
    (<EQUAL? ,HERE ,NORTH-OF-HOUSE>
      <PRINTI "North of House|You face the north side of a white house. The windows are boarded up.|">)
    (<EQUAL? ,HERE ,EAST-OF-HOUSE>
      <PRINTI "East of House|There is a small window here. It is slightly ajar, but this demo stops at the house.|">)
    (T <PRINTI "South of House|There is no door here, and all the windows are boarded.|">)>>
<ROUTINE MAIN ("AUX" W1 W2)
  <PUTB ,INBUF 0 78>
  <PUTB ,LEXBUF 0 16>
  <PRINTI "ZORK I: West of House demo|Type HELP for commands.||">
  <DESCRIBE>
  <REPEAT ()
    <PRINTI "> ">
    <READ ,INBUF ,LEXBUF>
    <SET W1 <GET ,LEXBUF 1>>
    <SET W2 <GET ,LEXBUF 3>>
    <COND (<EQUAL? .W1 ,W?GO> <SET W1 .W2> <SET W2 0>)>
    <SETG TURNS <+ ,TURNS 1>>
    <COND
      (<EQUAL? .W1 ,W?LOOK ,W?L> <DESCRIBE>)
      (<EQUAL? .W1 ,W?HELP> <PRINTI "LOOK, NORTH, SOUTH, EAST, WEST, OPEN MAILBOX, CLOSE MAILBOX, TAKE LEAFLET, READ LEAFLET, INVENTORY, QUIT.|">)
      (<EQUAL? .W1 ,W?QUIT> <PRINTI "Goodbye.|"> <QUIT>)
      (<EQUAL? .W1 ,W?INVENTORY ,W?I>
        <COND (,HAVE-LEAFLET <PRINTI "You have a leaflet.|">)
              (T <PRINTI "You are empty-handed.|">)>)
      (<EQUAL? .W1 ,W?OPEN>
        <COND (<NOT <EQUAL? ,HERE ,WEST-OF-HOUSE>> <PRINTI "There is no mailbox here.|">)
              (<EQUAL? .W2 ,W?MAILBOX ,W?BOX> <SETG BOX-OPEN T> <PRINTI "Opening the small mailbox reveals a leaflet.|">)
              (T <PRINTI "Open what?|">)>)
      (<EQUAL? .W1 ,W?CLOSE>
        <COND (<AND <EQUAL? ,HERE ,WEST-OF-HOUSE> <EQUAL? .W2 ,W?MAILBOX ,W?BOX>> <SETG BOX-OPEN <>> <PRINTI "The mailbox is closed.|">)
              (T <PRINTI "There is no mailbox here.|">)>)
      (<EQUAL? .W1 ,W?TAKE ,W?GET>
        <COND (<NOT <EQUAL? .W2 ,W?LEAFLET>> <PRINTI "Take what?|">)
              (<NOT <EQUAL? ,HERE ,WEST-OF-HOUSE>> <PRINTI "You cannot see a leaflet here.|">)
              (<NOT ,BOX-OPEN> <PRINTI "The mailbox is closed.|">)
              (,HAVE-LEAFLET <PRINTI "You already have it.|">)
              (<EQUAL? .W2 ,W?LEAFLET> <SETG HAVE-LEAFLET T> <PRINTI "Taken.|">)
              (T <PRINTI "Take what?|">)>)
      (<EQUAL? .W1 ,W?READ>
        <COND (<AND <EQUAL? .W2 ,W?LEAFLET> <OR ,HAVE-LEAFLET <AND <EQUAL? ,HERE ,WEST-OF-HOUSE> ,BOX-OPEN>>>
                 <PRINTI "WELCOME TO ZORK! ZORK is a game of adventure, danger, and low cunning.|">)
              (T <PRINTI "You cannot see a leaflet here.|">)>)
      (<EQUAL? .W1 ,W?NORTH ,W?N>
        <COND (<EQUAL? ,HERE ,WEST-OF-HOUSE ,EAST-OF-HOUSE> <SETG HERE ,NORTH-OF-HOUSE> <DESCRIBE>)
              (T <PRINTI "You cannot go that way.|">)>)
      (<EQUAL? .W1 ,W?SOUTH ,W?S>
        <COND (<EQUAL? ,HERE ,WEST-OF-HOUSE ,EAST-OF-HOUSE> <SETG HERE ,SOUTH-OF-HOUSE> <DESCRIBE>)
              (T <PRINTI "You cannot go that way.|">)>)
      (<EQUAL? .W1 ,W?EAST ,W?E>
        <COND (<EQUAL? ,HERE ,NORTH-OF-HOUSE ,SOUTH-OF-HOUSE> <SETG HERE ,EAST-OF-HOUSE> <DESCRIBE>)
              (T <PRINTI "The door is boarded and you cannot remove the boards.|">)>)
      (<EQUAL? .W1 ,W?WEST ,W?W>
        <COND (<EQUAL? ,HERE ,NORTH-OF-HOUSE ,SOUTH-OF-HOUSE> <SETG HERE ,WEST-OF-HOUSE> <DESCRIBE>)
              (T <PRINTI "The forest path is outside this demo.|">)>)
      (T <PRINTI "I do not understand that command.|">)>>>

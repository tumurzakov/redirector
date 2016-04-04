Redirector
----------

Proxy server that prevent time waste during working hours

 $ ./proxy --help
 Usage of ./proxy:
   -blacklist="blacklist": File that contains a list of blocking urls(regexp)
   -blockmode=false: Default blocking
   -hours="": Working hours, example: 8-11,13-17
   -orgdir="": Orgmode directory to parse clocking instructions
   -proxy=":8080": Proxy listen address
   -web=":8081": Proxy listen address


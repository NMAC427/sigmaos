message(STATUS "Building python clntlib library")

add_library(clntlib SHARED clntlib.cc) 

target_link_libraries(clntlib PRIVATE 
  sigmap_proxy 
  serr
  util_log
  rpc 
  proc
  proc_proto
  sigmap_proxy_proto
  io_conn
  io_conn_unix
  io_frame
  io_transport
  io_demux
  /usr/lib/libprotobuf.so
)

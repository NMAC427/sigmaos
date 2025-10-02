message(STATUS "Building spawn-latency-cpp proc")

add_executable(echo-srv-cpp
  main.cc
)

target_link_libraries(echo-srv-cpp PRIVATE 
  sigmaos_core
  echo_proto
  app_echo
)

target_include_directories(echo-srv-cpp PRIVATE ${CMAKE_SOURCE_DIR})

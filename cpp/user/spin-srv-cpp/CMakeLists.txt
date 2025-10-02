message(STATUS "Building spin-srv-cpp proc")

add_executable(spin-srv-cpp
  main.cc
)

target_link_libraries(spin-srv-cpp PRIVATE 
  sigmaos_core
  spin_proto
  app_spin
  epcache_proto
  app_epcache
  cache_proto
  app_cache
)

target_include_directories(spin-srv-cpp PRIVATE ${CMAKE_SOURCE_DIR})

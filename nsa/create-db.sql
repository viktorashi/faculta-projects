CREATE DATABASE IF NOT EXISTS `satelites`;
USE `satelites`;

CREATE TABLE IF NOT EXISTS `satelite` (
  `id` INT NOT NULL AUTO_INCREMENT,
  `name` TEXT NOT NULL,
  `semimajor_axis` DOUBLE NOT NULL,
  `eccentricity` DOUBLE NOT NULL,
  `inclination` DOUBLE NOT NULL,
  `longitude_ascending_node` DOUBLE NOT NULL,
  `argument_of_perigee` DOUBLE NOT NULL,
  PRIMARY KEY (`id`)
) ENGINE = InnoDB;
